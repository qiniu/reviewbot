package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	authorizationv1 "k8s.io/api/authorization/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

var (
	ErrPermissionDenied    = errors.New("permission check failed: not allowed to create Job in the specified namespace")
	ErrInvalidCopySSHKey   = errors.New("invalid copy ssh key format")
	ErrUnexpectedPodStatus = errors.New("unexpected pod status")
)

// KubernetesRunner is a runner that runs the linter in a Kubernetes pod.
type KubernetesRunner struct {
	client *kubernetes.Clientset
	// script is the final script to be executed
	script string
}

func NewKubernetesRunner(kubeConfig string) (Runner, error) {
	if kubeConfig == "" {
		kubeConfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	_, err := os.Stat(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("KubeConfigPath is not found :%w", err)
	}
	config, err := loadClusterConfig("", kubeConfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &KubernetesRunner{client: clientset}, nil
}

func (k *KubernetesRunner) Run(ctx context.Context, cfg *config.Linter) (io.ReadCloser, error) {
	log := lintersutil.FromContext(ctx)
	newCfg, err := cfg.Modifier.Modify(cfg)
	if err != nil {
		return nil, err
	}
	log.Infof("final config: %v", newCfg)

	scriptContent := ""
	// handle args
	scriptContent += strings.Join(newCfg.Args, " ")
	log.Infof("Script content: \n%s", scriptContent)
	k.script = scriptContent

	// create script configmap
	cmName := strings.ToLower(fmt.Sprintf("%s-%s-%s-%d-%s", cfg.Org, cfg.Repo, cfg.Name, cfg.Number, log.ReqId))
	scriptConfigMap, err := k.createScriptConfigMap(ctx, cfg, cmName, scriptContent)
	if err != nil {
		return nil, err
	}

	jobName := strings.ToLower(fmt.Sprintf("%s-%s-%s-%d-%s", cfg.Org, cfg.Repo, cfg.Name, cfg.Number, log.ReqId))
	job := k.newJob(cfg, jobName, scriptConfigMap.Name)
	containerName := job.Spec.Template.Spec.Containers[0].Name

	var srcPath, dstPath string
	if cfg.KubernetesAsRunner.CopySSHKeyToPod != "" {
		paths := strings.Split(cfg.KubernetesAsRunner.CopySSHKeyToPod, ":")
		switch len(paths) {
		case 2:
			srcPath, dstPath = paths[0], paths[1]
		case 1:
			srcPath, dstPath = paths[0], paths[0]
		default:
			log.Errorf("invalid copy ssh key format: %s", cfg.KubernetesAsRunner.CopySSHKeyToPod)
			return nil, ErrInvalidCopySSHKey
		}

		log.Infof("copy ssh key to pod: src: %s, dst: %s", srcPath, dstPath)

		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "ssh-mount",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "ssh-mount",
			MountPath: filepath.Dir(dstPath),
		})
	}

	createdJob, err := k.createOrRecreateJobAndWaitForPod(ctx, job, cfg.KubernetesAsRunner.Namespace)
	if err != nil {
		return nil, err
	}

	podName := k.getPodName(ctx, cfg.KubernetesAsRunner.Namespace, createdJob.Name)
	namespace := cfg.KubernetesAsRunner.Namespace

	// copy ssh to pod
	if cfg.KubernetesAsRunner.CopySSHKeyToPod != "" {
		err = copyToPod(ctx, namespace, podName, containerName, srcPath, dstPath)
		if err != nil {
			log.Errorf("failed to copy ssh key to pod: %v", err)
			return nil, err
		}
	}

	// copy repo code to pod
	err = k.copyCodeToPod(ctx, cfg.WorkDir, namespace, podName, cfg.WorkDir)
	if err != nil {
		log.Errorf("failed to copy code to pod: %v", err)
		return nil, err
	}

	// wait for job completion
	err = k.waitForJobCompletion(ctx, cfg.KubernetesAsRunner.Namespace, createdJob.Name)
	if err != nil {
		log.Errorf("wait for job completion failed: %v", err)
		return nil, err
	}

	return k.getPodLogs(ctx, cfg.KubernetesAsRunner.Namespace, podName)
}

func (k *KubernetesRunner) GetFinalScript() string {
	return k.script
}

// check the permission to create pod in the namespace.
func (k *KubernetesRunner) Prepare(ctx context.Context, cfg *config.Linter) error {
	log := lintersutil.FromContext(ctx)
	ssar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: cfg.KubernetesAsRunner.Namespace,
				Verb:      "create",
				Group:     "batch", // "batch" API group for Job
				Resource:  "jobs",  // Resource type is jobs
			},
		},
	}

	authClient := k.client.AuthorizationV1().SelfSubjectAccessReviews()
	response, err := authClient.Create(ctx, ssar, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("failed to check permission: %v", err)
		return err
	}

	if !response.Status.Allowed {
		return ErrPermissionDenied
	}

	return nil
}

func (k *KubernetesRunner) Clone() Runner {
	return &KubernetesRunner{client: k.client}
}

func loadClusterConfig(masterURL, kubeConfig string) (*rest.Config, error) {
	clusterConfig, err := clientcmd.BuildConfigFromFlags(masterURL, kubeConfig)
	if err == nil {
		return clusterConfig, nil
	}

	credentials, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, fmt.Errorf("could not load credentials from config: %w", err)
	}

	clusterConfig, err = clientcmd.NewDefaultClientConfig(*credentials, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not load client configuration: %w", err)
	}
	return clusterConfig, nil
}

func int32Ptr(i int32) *int32 {
	return &i
}
func (k *KubernetesRunner) copyCodeToPod(ctx context.Context, srcPath, namespace, podName, destPath string) error {
	log := lintersutil.FromContext(ctx)
	// ensure srcPath ends with "/" so that it copies the directory contents instead of the directory itself
	if !strings.HasSuffix(srcPath, "/.") {
		srcPath += "/."
	}

	log.Infof("copy code to pod: src: %s, dest: %s", srcPath, destPath)
	cmd := exec.CommandContext(ctx, "kubectl", "cp", srcPath, fmt.Sprintf("%s/%s:%s", namespace, podName, destPath), "-c", "wait-for-code")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Errorf("failed to copy code to pod: %v, output: %s, command: %s", err, string(out), cmd.Args)
		return err
	}

	// create a marker file to indicate that the code has been copied
	cmd = exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "-c", "wait-for-code", "--", "touch", "/workspace/.code-copied")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Errorf("failed to create code-copied marker file: %v, output: %s, command: %s", err, string(out), cmd.Args)
		return err
	}
	return nil
}

func copyToPod(ctx context.Context, namespace, podName, containerName, srcPath, dstPath string) error {
	log := lintersutil.FromContext(ctx)
	cmd := exec.CommandContext(ctx, "kubectl", "cp", srcPath, fmt.Sprintf("%s/%s:%s", namespace, podName, dstPath), "-c", containerName)
	log.Infof("Executing command: %s\n", cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file to pod: %v", err)
	}

	return nil
}

func (k *KubernetesRunner) waitForJobCompletion(ctx context.Context, namespace, jobName string) error {
	return wait.PollUntilContextTimeout(ctx, time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		job, err := k.client.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
			return true, nil
		}
		return false, nil
	})
}

func (k *KubernetesRunner) createScriptConfigMap(ctx context.Context, cfg *config.Linter, name, scriptContent string) (*corev1.ConfigMap, error) {
	_, err := k.client.CoreV1().ConfigMaps(cfg.KubernetesAsRunner.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		err = k.client.CoreV1().ConfigMaps(cfg.KubernetesAsRunner.Namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			return nil, err
		}
	} else if !k8serrors.IsNotFound(err) {
		return nil, err
	}

	// create new configmap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cfg.KubernetesAsRunner.Namespace,
		},
		Data: map[string]string{
			"script.sh": scriptContent,
		},
	}

	return k.client.CoreV1().ConfigMaps(cfg.KubernetesAsRunner.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
}

func (k *KubernetesRunner) getPodLogs(ctx context.Context, namespace, podName string) (io.ReadCloser, error) {
	logs, err := k.client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{}).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(logs)), nil
}

func (k *KubernetesRunner) getPodName(ctx context.Context, namespace string, jobName string) string {
	log := lintersutil.FromContext(ctx)

	labelSelector := "job-name=" + jobName
	pods, err := k.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Errorf("failed to get pod list: %v", err)
		return ""
	}

	if len(pods.Items) > 0 {
		return pods.Items[0].Name
	}

	log.Errorf("no pod found for job: %s", jobName)
	return ""
}

func (k *KubernetesRunner) createOrRecreateJobAndWaitForPod(ctx context.Context, job *batchv1.Job, namespace string) (*batchv1.Job, error) {
	log := lintersutil.FromContext(ctx)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var createdJob *batchv1.Job
	_, err := k.client.BatchV1().Jobs(namespace).Get(ctx, job.Name, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
	} else {
		log.Infof("job %s already exists, delete it first", job.Name)
		deletePolicy := metav1.DeletePropagationForeground
		deleteOptions := metav1.DeleteOptions{PropagationPolicy: &deletePolicy}
		err = k.client.BatchV1().Jobs(namespace).Delete(ctx, job.Name, deleteOptions)
		if err != nil {
			return nil, err
		}

		// wait for job to be deleted
		err = wait.PollUntilContextTimeout(ctx, time.Second, time.Minute, true, func(ctx context.Context) (bool, error) {
			_, err := k.client.BatchV1().Jobs(namespace).Get(ctx, job.Name, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
		if err != nil {
			return nil, err
		}
	}

	err = retry.OnError(retry.DefaultRetry, k8serrors.IsServerTimeout, func() error {
		createdJob, err = k.client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
		return err
	})
	if err != nil {
		return nil, err
	}

	log.Infof("created job: %s", createdJob.Name)

	// wait for pod to be initialized
	var pod corev1.Pod
	err = wait.PollUntilContextTimeout(ctx, time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		podList, err := k.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "job-name=" + createdJob.Name,
		})
		if err != nil {
			return false, err
		}

		if len(podList.Items) == 0 {
			return false, nil
		}

		pod = podList.Items[0]

		// check init container status
		for _, initStatus := range pod.Status.InitContainerStatuses {
			if initStatus.State.Running != nil {
				return true, nil
			}
		}

		// if pod status is abnormal, return error
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodUnknown {
			log.Errorf("unexpected pod status: %s", pod.Status.Phase)
			return false, ErrUnexpectedPodStatus
		}

		// pod is still waiting or initializing
		return false, nil
	})

	if err != nil {
		log.Errorf("failed to wait for pod to be ready: %v", err)
		return nil, err
	}

	log.Infof("pod of job: %s is ready", pod.GetName())

	return createdJob, nil
}

func (k *KubernetesRunner) newJob(cfg *config.Linter, jobName, scriptConfigMapName string) *batchv1.Job {
	currentTime := time.Now().UTC().Format(time.RFC3339)
	// see https://github.com/kubefree/kubefree
	kubefreeLabel := "sleepmode.kubefree.com/delete-after"
	kubefreeAnnotation := "sleepmode.kubefree.com/activity-status"
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cfg.KubernetesAsRunner.Namespace,
			Name:      jobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kubefreeLabel: "24h",
					},
					Annotations: map[string]string{
						kubefreeAnnotation: fmt.Sprintf(`{"LastActivityTime": "%s"}`, currentTime),
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "code-mount",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "script-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: scriptConfigMapName,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "script.sh",
											Path: "script.sh",
										},
									},
									DefaultMode: int32Ptr(0755),
								},
							},
						},
						{
							Name: "code-flag",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:       "wait-for-code",
							Image:      cfg.KubernetesAsRunner.Image,
							Command:    []string{"sh", "-c", "while [ ! -f /workspace/.code-copied ]; do sleep 1; done"},
							WorkingDir: cfg.WorkDir,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "code-mount",
									MountPath: cfg.WorkDir,
								},
								{
									Name:      "code-flag",
									MountPath: "/workspace",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:       "linter",
							Image:      cfg.KubernetesAsRunner.Image,
							Command:    []string{"/bin/sh", "-c"},
							Args:       []string{"/scripts/script.sh"},
							WorkingDir: cfg.WorkDir,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "code-mount",
									MountPath: cfg.WorkDir,
								},
								{
									Name:      "script-config",
									MountPath: "/scripts/script.sh",
									SubPath:   "script.sh",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			BackoffLimit: int32Ptr(0),
		},
	}
}
