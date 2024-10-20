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

type KubernetesRunner struct {
	client *kubernetes.Clientset
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

	job := k.newJob(cfg)
	job.Name = strings.ToLower(fmt.Sprintf("%s-%s-%s-%d-%s", cfg.Org, cfg.Repo, cfg.Name, cfg.Number, log.ReqId))
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
	copyPath := filepath.Dir(cfg.WorkDir)
	err = copyToPod(ctx, namespace, podName, containerName, copyPath+"/.", copyPath)
	if err != nil {
		log.Errorf("failed to copy code to pod :%v", err)
		return nil, err
	}

	// execute command
	err = k.execCommandOnPod(ctx, namespace, podName, containerName, cfg.WorkDir, scriptContent)
	if err != nil {
		log.Errorf("failed to exec command to pod :%v", err)
		return nil, err
	}

	logs, err := k.client.CoreV1().Pods(cfg.KubernetesAsRunner.Namespace).GetLogs(podName, &corev1.PodLogOptions{}).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(logs)), nil
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

func (k *KubernetesRunner) execCommandOnPod(ctx context.Context, namespace, podName, containerName, workDir, commandStr string) error {
	log := lintersutil.FromContext(ctx)

	// create command script on pod
	cmd := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "-c", containerName, "--", "bash", "-c", fmt.Sprintf("echo '%s' > %s/script.sh && chmod +x %s/script.sh", commandStr, workDir, workDir))
	log.Infof("Executing command: %s\n", cmd.Args)

	output, execErr := cmd.CombinedOutput()
	if execErr != nil {
		log.Errorf("Error executing command,marked and continue: %v\n, output:\n%s\n", execErr, output)
		// just marked and continue
	}

	// exec command script
	c := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "-c", containerName, "--", "bash", "-c", fmt.Sprintf("bash %s/script.sh  > /proc/1/fd/1", workDir))
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	if err != nil {
		log.Errorf("Error executing command, marked and continue, err: %v, output:\n%s\n", err, b.String())
		// just marked and continue
	}
	return nil
}

func (k *KubernetesRunner) newJob(cfg *config.Linter) *batchv1.Job {
	currentTime := time.Now().UTC().Format(time.RFC3339)
	// see https://github.com/kubefree/kubefree
	kubefreeLabel := "sleepmode.kubefree.com/delete-after"
	kubefreeAnnotation := "sleepmode.kubefree.com/activity-status"

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cfg.KubernetesAsRunner.Namespace,
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
					Containers: []corev1.Container{
						{
							Name:       "runner",
							Image:      cfg.KubernetesAsRunner.Image,
							Command:    []string{"/bin/sh", "-c"},
							Args:       []string{"sleep 3600"},
							WorkingDir: cfg.WorkDir,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "code-dir",
									MountPath: cfg.WorkDir,
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "code-dir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
			BackoffLimit: int32Ptr(0),
		},
	}
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
		// job already exists, delete it first
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

	// wait for pod to be running
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
		switch pod.Status.Phase {
		case corev1.PodRunning:
			return true, nil
		case corev1.PodFailed, corev1.PodSucceeded, corev1.PodUnknown:
			log.Errorf("unexpected pod status: %s", pod.Status.Phase)
			return false, ErrUnexpectedPodStatus
		case corev1.PodPending:
			return false, nil
		default:
			return false, nil
		}
	})

	if err != nil {
		log.Errorf("failed to wait for pod to be running: %v", err)
		return nil, err
	}

	log.Infof("pod of job: %s is running", pod.GetName())

	return createdJob, nil
}
