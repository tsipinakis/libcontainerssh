package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"time"

	containerSSHConfig "github.com/containerssh/libcontainerssh/config"
	"github.com/containerssh/libcontainerssh/internal/metrics"
	"github.com/containerssh/libcontainerssh/internal/structutils"
	"github.com/containerssh/libcontainerssh/log"
	"github.com/containerssh/libcontainerssh/message"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

type kubernetesClientImpl struct {
	config                containerSSHConfig.KubernetesConfig
	logger                log.Logger
	client                *kubernetes.Clientset
	restClient            *restclient.RESTClient
	connectionConfig      *restclient.Config
	backendRequestsMetric metrics.SimpleCounter
	backendFailuresMetric metrics.SimpleCounter
}

func (k *kubernetesClientImpl) createPod(
	ctx context.Context,
	labels map[string]string,
	annotations map[string]string,
	env map[string]string,
	tty *bool,
	cmd []string,
) (kubePod kubernetesPod, lastError error) {
	podConfig, err := k.getPodConfig(tty, cmd, labels, annotations, env)
	if err != nil {
		return nil, err
	}
	logger := k.logger

	logger.Debug(message.NewMessage(message.MKubernetesPodCreate, "Creating pod"))
loop:
	for {
		kubePod, lastError = k.attemptPodCreate(ctx, podConfig, logger, tty)
		if lastError == nil {
			return kubePod, nil
		}
		select {
		case <-ctx.Done():
			break loop
		case <-time.After(10 * time.Second):
		}
	}
	err = message.WrapUser(
		lastError,
		message.EKubernetesFailedPodCreate,
		UserMessageInitializeSSHSession,
		"Failed to create pod, giving up",
	)
	logger.Error(err)
	return nil, err
}

func (k *kubernetesClientImpl) getPodList(
	ctx context.Context,
	filter_labels map[string]string,
) ([]core.Pod, error) {
	selector := labels.NewSelector()
	for k, v := range filter_labels {
		label, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return nil, err
		}
		selector.Add(*label)
	}
	result, err := k.client.CoreV1().Pods(k.config.Pod.Metadata.Namespace).List(ctx, meta.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	if result.Continue != "" || len(result.Items) > 1 {
		return nil, fmt.Errorf("Found more than 1 pod matching the labels")
	}
	if len(result.Items) == 0 {
		return nil, nil
	}
	return result.Items, nil
}

func (k *kubernetesClientImpl) k8sPodToPodImpl(pod core.Pod) kubernetesPodImpl {
	return kubernetesPodImpl{
		pod: &pod,
		client: k.client,
		restClient: k.restClient,
		config: k.config,
		logger: k.logger.WithLabel("podName", pod.Name),
		// tty ?
		connectionConfig: k.connectionConfig,
		backendRequestsMetric: k.backendRequestsMetric,
		backendFailuresMetric: k.backendFailuresMetric,
		lock:                  &sync.Mutex{},
		wg:                    &sync.WaitGroup{},
		removeLock:            &sync.Mutex{},
	}
}

func (k *kubernetesClientImpl) findPod(
	ctx context.Context,
	filter_labels map[string]string,
	connectionId string,
) (kubernetesPod, error) {
	tries := 0
	for {
		if tries > 3 {
			return nil, fmt.Errorf("Tries exhausted")
		}
		podList, err := k.getPodList(ctx, filter_labels)
		if err != nil {
			return nil, err
		}
		for _, pod := range podList {
			_, ok := pod.Annotations["containerssh.io/coordination"]
			if !ok {
				// should never happen
				k.logger.Debug("No coordination found")
				continue
			}
			podImpl := k.k8sPodToPodImpl(pod)
			err := podImpl.markInUse(ctx, connectionId)
			if err != nil {
				k.logger.Debug(err)
				continue
			}
			return &podImpl, nil
		}
		tries++
	}
}

func (k *kubernetesClientImpl) attemptPodCreate(
	ctx context.Context,
	podConfig containerSSHConfig.KubernetesPodConfig,
	logger log.Logger,
	tty *bool,
) (kubernetesPod, error) {
	var pod *core.Pod
	var lastError error
	k.backendRequestsMetric.Increment()
	pod, lastError = k.client.CoreV1().Pods(podConfig.Metadata.Namespace).Create(
		ctx,
		&core.Pod{
			ObjectMeta: podConfig.Metadata,
			Spec:       podConfig.Spec,
		},
		meta.CreateOptions{},
	)
	if lastError == nil {
		createdPod := &kubernetesPodImpl{
			pod:                   pod,
			client:                k.client,
			restClient:            k.restClient,
			config:                k.config,
			logger:                logger.WithLabel("podName", pod.Name),
			tty:                   tty,
			connectionConfig:      k.connectionConfig,
			backendRequestsMetric: k.backendRequestsMetric,
			backendFailuresMetric: k.backendFailuresMetric,
			lock:                  &sync.Mutex{},
			wg:                    &sync.WaitGroup{},
			removeLock:            &sync.Mutex{},
		}
		return createdPod.wait(ctx)
	}
	k.backendFailuresMetric.Increment()
	logger.Debug(
		message.Wrap(
			lastError,
			message.EKubernetesFailedPodCreate,
			"Failed to create pod, retrying in 10 seconds",
		),
	)
	return nil, lastError
}

func (k *kubernetesClientImpl) getPodConfig(
	tty *bool,
	cmd []string,
	labels map[string]string,
	annotations map[string]string,
	env map[string]string,
) (
	containerSSHConfig.KubernetesPodConfig,
	error,
) {
	var podConfig containerSSHConfig.KubernetesPodConfig
	if err := structutils.Copy(&podConfig, k.config.Pod); err != nil {
		return containerSSHConfig.KubernetesPodConfig{}, err
	}

	if podConfig.Mode == containerSSHConfig.KubernetesExecutionModeSession {
		if tty != nil {
			podConfig.Spec.Containers[k.config.Pod.ConsoleContainerNumber].TTY = *tty
		}
		podConfig.Spec.Containers[k.config.Pod.ConsoleContainerNumber].Stdin = true
		podConfig.Spec.Containers[k.config.Pod.ConsoleContainerNumber].StdinOnce = true
		if !podConfig.DisableAgent {
			podConfig.Spec.Containers[k.config.Pod.ConsoleContainerNumber].Command = append(
				[]string{
					podConfig.AgentPath,
					"console",
					"--wait",
					"--pid",
					"--",
				},
				cmd...,
			)
		} else {
			podConfig.Spec.Containers[k.config.Pod.ConsoleContainerNumber].Command = cmd
		}
		if podConfig.Spec.RestartPolicy == "" {
			podConfig.Spec.RestartPolicy = core.RestartPolicyNever
		}
	} else {
		podConfig.Spec.Containers[k.config.Pod.ConsoleContainerNumber].Command = k.config.Pod.IdleCommand
	}

	k.addLabelsToPodConfig(&podConfig, labels)
	k.addAnnotationsToPodConfig(&podConfig, annotations)
	k.addEnvToPodConfig(env, &podConfig)
	return podConfig, nil
}

func (k *kubernetesClientImpl) addLabelsToPodConfig(podConfig *containerSSHConfig.KubernetesPodConfig, labels map[string]string) {
	if podConfig.Metadata.Labels == nil {
		podConfig.Metadata.Labels = map[string]string{}
	}
	for k, v := range labels {
		podConfig.Metadata.Labels[k] = v
	}
}

func (k *kubernetesClientImpl) addAnnotationsToPodConfig(podConfig *containerSSHConfig.KubernetesPodConfig, annotations map[string]string) {
	if podConfig.Metadata.Annotations == nil {
		podConfig.Metadata.Annotations = map[string]string{}
	}
	for k, v := range annotations {
		podConfig.Metadata.Annotations[k] = v
	}
}

func (k *kubernetesClientImpl) addEnvToPodConfig(env map[string]string, podConfig *containerSSHConfig.KubernetesPodConfig) {
	for key, value := range env {
		podConfig.Spec.Containers[k.config.Pod.ConsoleContainerNumber].Env = append(
			podConfig.Spec.Containers[k.config.Pod.ConsoleContainerNumber].Env,
			core.EnvVar{
				Name:  key,
				Value: value,
			},
		)
	}
}
