package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/qiniu/reviewbot/config"
	"github.com/qiniu/reviewbot/internal/lintersutil"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubenertesRunner struct {
	Cli    *kubernetes.Clientset
	script string
	// config *rest.Config
}

func NewKubenertesRunner(kubeConfig string) (Runner, error) {
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
	return &KubenertesRunner{Cli: clientset}, nil
}

func (k *KubenertesRunner) Run(ctx context.Context, cfg *config.Linter) (io.ReadCloser, error) {
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

	// create default config
	// Note(wwcchh0123): support unmarshalling YAML into structures in the future
	job := k.initDefaultPodConfig(cfg)
	containerName := job.Spec.Template.Spec.Containers[0].Name

	var srcPath, dstPath string
	if cfg.KubernetesAsRunner.SSHKeyMount != "" {
		paths := strings.Split(cfg.KubernetesAsRunner.SSHKeyMount, ":")
		if len(paths) == 2 {
			srcPath, dstPath = paths[0], paths[1]
		} else if len(paths) == 1 {
			srcPath, dstPath = paths[0], paths[0]
		} else {
			return nil, fmt.Errorf("invalid copy ssh key format: %s", cfg.DockerAsRunner.CopySSHKeyToContainer)
		}

		log.Infof("copy ssh key to container: src: %s, dst: %s", srcPath, dstPath)

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

	createdjob, err := k.Cli.BatchV1().Jobs(cfg.KubernetesAsRunner.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("create pod failed: %v", err)
		return nil, err
	}

	podName := k.getJobNameWithPrefix(ctx, cfg.KubernetesAsRunner.Namespace, createdjob.Name)
	namespace := cfg.KubernetesAsRunner.Namespace

	// copy ssh to pod
	err = copyToPod(ctx, namespace, podName, containerName, srcPath, dstPath)
	if err != nil {
		log.Errorf("failed to copy ssh key to container: %v", err)
		return nil, err
	}

	// copy repo code to pod
	copyPath := filepath.Dir(cfg.WorkDir)
	err = copyToPod(ctx, namespace, podName, containerName, copyPath+"/.", copyPath)
	if err != nil {
		log.Errorf("failed to copy to pod :%v", err)
		return nil, err
	}

	// execute command
	err = k.execCommandOnPod(ctx, namespace, podName, containerName, cfg.WorkDir, scriptContent)
	if err != nil {
		log.Errorf("failed to exec commannd to pod :%v", err)
		return nil, err
	}

	logs, err := k.Cli.CoreV1().Pods(cfg.KubernetesAsRunner.Namespace).GetLogs(podName, &corev1.PodLogOptions{}).Do(ctx).Raw()
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(logs)), nil
}

func (k *KubenertesRunner) GetFinalScript() string {
	return k.script
}

func (k *KubenertesRunner) Prepare(ctx context.Context, cfg *config.Linter) error {
	// check kube config is working
	_, err := k.Cli.CoreV1().Namespaces().Get(ctx, cfg.KubernetesAsRunner.Namespace, metav1.GetOptions{})
	if err != nil {
		return err
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
	log.Infof("Executing command: %s %s\n", cmd.Path, cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy file to pod: %v", err)
	}

	return nil
}

func (k *KubenertesRunner) getJobNameWithPrefix(ctx context.Context, namespace string, prefix string) string {
	time.Sleep(10 * time.Second)
	jobs, err := k.Cli.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Failed to list jobs: %v", err)
	}

	for _, job := range jobs.Items {
		if strings.HasPrefix(job.Name, prefix) {
			return job.Name
		}
	}
	return ""
}

func (k *KubenertesRunner) execCommandOnPod(ctx context.Context, namespace, podName, containerName, workDir, commandStr string) error {
	log := lintersutil.FromContext(ctx)

	// create command script on pod
	cmd := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "-c", containerName, "--", "bash", "-c", fmt.Sprintf("echo '%s' > %s/script.sh && chmod +x %s/script.sh", commandStr, workDir, workDir))
	log.Infof("Executing command: %s %s\n", cmd.Path, cmd.Args)

	_, execErr := cmd.CombinedOutput()
	if execErr != nil {
		log.Errorf("Error executing command,marked and continue: %v\n", execErr)
		return execErr
	}

	// exec command script
	c := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "-c", containerName, "--", "bash", "-c", fmt.Sprintf("bash %s/script.sh  > /proc/1/fd/1", workDir))
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	if err != nil {
		log.Errorf("Error executing command,marked and continue: %v\n", err)
	}
	return nil
}

func (k *KubenertesRunner) initDefaultPodConfig(cfg *config.Linter) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid.New().String(),
			Namespace: cfg.KubernetesAsRunner.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
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
