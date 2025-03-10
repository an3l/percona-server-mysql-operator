package mysql

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	apiv1alpha1 "github.com/percona/percona-server-mysql-operator/api/v1alpha1"
	"github.com/percona/percona-server-mysql-operator/pkg/k8s"
	"github.com/percona/percona-server-mysql-operator/pkg/util"
)

const (
	componentName    = "mysql"
	dataVolumeName   = "datadir"
	DataMountPath    = "/var/lib/mysql"
	CustomConfigKey  = "my.cnf"
	configVolumeName = "config"
	configMountPath  = "/etc/mysql/config"
	credsVolumeName  = "users"
	CredsMountPath   = "/etc/mysql/mysql-users-secret"
	tlsVolumeName    = "tls"
	tlsMountPath     = "/etc/mysql/mysql-tls-secret"
)

const (
	DefaultPort      = 3306
	DefaultAdminPort = 33062
	DefaultXPort     = 33060
)

type User struct {
	Username apiv1alpha1.SystemUser
	Password string
	Hosts    []string
}

type Exposer apiv1alpha1.PerconaServerMySQL

func (e *Exposer) Exposed() bool {
	return e.Spec.MySQL.Expose.Enabled
}

func (e *Exposer) Name(index string) string {
	cr := apiv1alpha1.PerconaServerMySQL(*e)
	return Name(&cr) + "-" + index
}

func (e *Exposer) Size() int32 {
	return e.Spec.MySQL.Size
}

func (e *Exposer) Labels() map[string]string {
	cr := apiv1alpha1.PerconaServerMySQL(*e)
	return MatchLabels(&cr)
}

func (e *Exposer) Service(name string) *corev1.Service {
	cr := apiv1alpha1.PerconaServerMySQL(*e)
	return PodService(&cr, cr.Spec.MySQL.Expose.Type, name)
}

func Name(cr *apiv1alpha1.PerconaServerMySQL) string {
	return cr.Name + "-" + componentName
}

func NamespacedName(cr *apiv1alpha1.PerconaServerMySQL) types.NamespacedName {
	return types.NamespacedName{Name: Name(cr), Namespace: cr.Namespace}
}

func ServiceName(cr *apiv1alpha1.PerconaServerMySQL) string {
	return Name(cr)
}

func PrimaryServiceName(cr *apiv1alpha1.PerconaServerMySQL) string {
	return Name(cr) + "-primary"
}

func UnreadyServiceName(cr *apiv1alpha1.PerconaServerMySQL) string {
	return Name(cr) + "-unready"
}

func ConfigMapName(cr *apiv1alpha1.PerconaServerMySQL) string {
	return Name(cr)
}

func MatchLabels(cr *apiv1alpha1.PerconaServerMySQL) map[string]string {
	return util.SSMapMerge(cr.MySQLSpec().Labels,
		map[string]string{apiv1alpha1.ComponentLabel: componentName},
		cr.Labels())
}

func StatefulSet(cr *apiv1alpha1.PerconaServerMySQL, initImage, configHash string) *appsv1.StatefulSet {
	labels := MatchLabels(cr)
	spec := cr.MySQLSpec()
	replicas := spec.Size
	t := true

	annotations := make(map[string]string)
	annotations["percona.com/configuration-hash"] = configHash

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name(cr),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			ServiceName:          ServiceName(cr),
			VolumeClaimTemplates: volumeClaimTemplates(spec),
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					NodeSelector: cr.Spec.MySQL.NodeSelector,
					Tolerations:  cr.Spec.MySQL.Tolerations,
					InitContainers: []corev1.Container{
						{
							Name:            componentName + "-init",
							Image:           initImage,
							ImagePullPolicy: spec.ImagePullPolicy,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      dataVolumeName,
									MountPath: DataMountPath,
								},
								{
									Name:      credsVolumeName,
									MountPath: CredsMountPath,
								},
								{
									Name:      tlsVolumeName,
									MountPath: tlsMountPath,
								},
							},
							Command:                  []string{"/ps-init-entrypoint.sh"},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							SecurityContext:          spec.ContainerSecurityContext,
						},
					},
					Containers:       containers(cr),
					Affinity:         spec.GetAffinity(labels),
					ImagePullSecrets: spec.ImagePullSecrets,
					// TerminationGracePeriodSeconds: 30,
					RestartPolicy: corev1.RestartPolicyAlways,
					SchedulerName: "default-scheduler",
					DNSPolicy:     corev1.DNSClusterFirst,
					Volumes: append(
						[]corev1.Volume{
							{
								Name: credsVolumeName,
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: cr.InternalSecretName(),
									},
								},
							},
							{
								Name: tlsVolumeName,
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: cr.Spec.SSLSecretName,
									},
								},
							},
							{
								Name: configVolumeName,
								VolumeSource: corev1.VolumeSource{
									Projected: &corev1.ProjectedVolumeSource{
										Sources: []corev1.VolumeProjection{
											{
												ConfigMap: &corev1.ConfigMapProjection{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: ConfigMapName(cr),
													},
													Items: []corev1.KeyToPath{
														{
															Key:  CustomConfigKey,
															Path: "my-config.cnf",
														},
													},
													Optional: &t,
												},
											},
											{
												Secret: &corev1.SecretProjection{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: ConfigMapName(cr),
													},
													Items: []corev1.KeyToPath{
														{
															Key:  CustomConfigKey,
															Path: "my-secret.cnf",
														},
													},
													Optional: &t,
												},
											},
										},
									},
								},
							},
						},
						spec.SidecarVolumes...,
					),
					SecurityContext: spec.PodSecurityContext,
				},
			},
		},
	}
}

func volumeClaimTemplates(spec *apiv1alpha1.MySQLSpec) []corev1.PersistentVolumeClaim {
	pvcs := []corev1.PersistentVolumeClaim{
		k8s.PVC(dataVolumeName, spec.VolumeSpec),
	}
	for _, p := range spec.SidecarPVCs {
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: p.Name},
			Spec:       p.Spec,
		})
	}

	return pvcs
}

func UnreadyService(cr *apiv1alpha1.PerconaServerMySQL) *corev1.Service {
	labels := MatchLabels(cr)

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      UnreadyServiceName(cr),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Name: "mysql",
					Port: DefaultPort,
				},
				{
					Name: "mysql-admin",
					Port: DefaultAdminPort,
				},
				{
					Name: "mysqlx",
					Port: DefaultXPort,
				},
			},
			Selector:                 labels,
			PublishNotReadyAddresses: true,
		},
	}
}

func HeadlessService(cr *apiv1alpha1.PerconaServerMySQL) *corev1.Service {
	labels := MatchLabels(cr)

	serviceType := corev1.ServiceTypeClusterIP
	clusterIP := "None"
	if cr.Spec.MySQL.ReplicasServiceType != "" && cr.Spec.MySQL.ReplicasServiceType != corev1.ServiceTypeClusterIP {
		serviceType = cr.Spec.MySQL.ReplicasServiceType
		clusterIP = ""
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName(cr),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:      serviceType,
			ClusterIP: clusterIP,
			Ports: []corev1.ServicePort{
				{
					Name: "mysql",
					Port: DefaultPort,
				},
				{
					Name: "mysql-admin",
					Port: DefaultAdminPort,
				},
				{
					Name: "mysqlx",
					Port: DefaultXPort,
				},
			},
			Selector: labels,
		},
	}
}

func PodService(cr *apiv1alpha1.PerconaServerMySQL, t corev1.ServiceType, podName string) *corev1.Service {
	labels := MatchLabels(cr)
	labels[apiv1alpha1.ExposedLabel] = "true"

	selector := MatchLabels(cr)
	selector["statefulset.kubernetes.io/pod-name"] = podName

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     t,
			Selector: selector,
			Ports: []corev1.ServicePort{
				{
					Name: componentName,
					Port: DefaultPort,
				},
				{
					Name: componentName + "-admin",
					Port: DefaultAdminPort,
				},
				{
					Name: componentName + "x",
					Port: DefaultXPort,
				},
			},
		},
	}
}

func PrimaryService(cr *apiv1alpha1.PerconaServerMySQL) *corev1.Service {
	labels := MatchLabels(cr)
	selector := util.SSMapCopy(labels)
	selector[apiv1alpha1.MySQLPrimaryLabel] = "true"

	serviceType := corev1.ServiceTypeClusterIP
	if cr.Spec.MySQL.PrimaryServiceType != "" {
		serviceType = cr.Spec.MySQL.PrimaryServiceType
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      PrimaryServiceName(cr),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
			Ports: []corev1.ServicePort{
				{
					Name: "mysql",
					Port: DefaultPort,
				},
				{
					Name: "mysql-admin",
					Port: DefaultAdminPort,
				},
				{
					Name: "mysqlx",
					Port: DefaultXPort,
				},
			},
			Selector: selector,
		},
	}
}

func containers(cr *apiv1alpha1.PerconaServerMySQL) []corev1.Container {
	containers := []corev1.Container{mysqldContainer(cr)}
	if pmm := cr.PMMSpec(); pmm != nil && pmm.Enabled {
		c := pmmContainer(cr.Name, cr.Spec.SecretsName, pmm)
		containers = append(containers, c)
	}

	return appendUniqueContainers(containers, cr.MySQLSpec().Sidecars...)
}

func mysqldContainer(cr *apiv1alpha1.PerconaServerMySQL) corev1.Container {
	spec := cr.MySQLSpec()

	return corev1.Container{
		Name:            componentName,
		Image:           spec.Image,
		ImagePullPolicy: spec.ImagePullPolicy,
		Resources:       spec.Resources,
		Env: []corev1.EnvVar{
			{
				Name:  "MONITOR_HOST",
				Value: "%",
			},
			{
				Name:  "SERVICE_NAME",
				Value: ServiceName(cr),
			},
			{
				Name:  "SERVICE_NAME_UNREADY",
				Value: UnreadyServiceName(cr),
			},
			{
				Name:  "CLUSTER_HASH",
				Value: cr.ClusterHash(),
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "mysql",
				ContainerPort: DefaultPort,
			},
			{
				Name:          "mysql-admin",
				ContainerPort: DefaultAdminPort,
			},
			{
				Name:          "mysqlx",
				ContainerPort: DefaultXPort,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      dataVolumeName,
				MountPath: DataMountPath,
			},
			{
				Name:      credsVolumeName,
				MountPath: CredsMountPath,
			},
			{
				Name:      tlsVolumeName,
				MountPath: tlsMountPath,
			},
			{
				Name:      configVolumeName,
				MountPath: configMountPath,
			},
		},
		Command:                  []string{"/var/lib/mysql/ps-entrypoint.sh"},
		Args:                     []string{"mysqld"},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		SecurityContext:          spec.ContainerSecurityContext,
		StartupProbe: &corev1.Probe{
			Handler: corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{"/var/lib/mysql/bootstrap"},
				},
			},
			InitialDelaySeconds:           spec.StartupProbe.InitialDelaySeconds,
			TimeoutSeconds:                spec.StartupProbe.TimeoutSeconds,
			PeriodSeconds:                 spec.StartupProbe.PeriodSeconds,
			FailureThreshold:              spec.StartupProbe.FailureThreshold,
			SuccessThreshold:              spec.StartupProbe.SuccessThreshold,
			TerminationGracePeriodSeconds: spec.StartupProbe.TerminationGracePeriodSeconds,
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{"/var/lib/mysql/healthcheck", "liveness"},
				},
			},
			InitialDelaySeconds:           spec.LivenessProbe.InitialDelaySeconds,
			TimeoutSeconds:                spec.LivenessProbe.TimeoutSeconds,
			PeriodSeconds:                 spec.LivenessProbe.PeriodSeconds,
			FailureThreshold:              spec.LivenessProbe.FailureThreshold,
			SuccessThreshold:              spec.LivenessProbe.SuccessThreshold,
			TerminationGracePeriodSeconds: spec.LivenessProbe.TerminationGracePeriodSeconds,
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{"/var/lib/mysql/healthcheck", "readiness"},
				},
			},
			InitialDelaySeconds:           spec.ReadinessProbe.InitialDelaySeconds,
			TimeoutSeconds:                spec.ReadinessProbe.TimeoutSeconds,
			PeriodSeconds:                 spec.ReadinessProbe.PeriodSeconds,
			FailureThreshold:              spec.ReadinessProbe.FailureThreshold,
			SuccessThreshold:              spec.ReadinessProbe.SuccessThreshold,
			TerminationGracePeriodSeconds: spec.ReadinessProbe.TerminationGracePeriodSeconds,
		},
	}
}

func pmmContainer(clusterName, secretsName string, pmmSpec *apiv1alpha1.PMMSpec) corev1.Container {
	ports := []corev1.ContainerPort{{ContainerPort: 7777}}
	for port := 30100; port <= 30105; port++ {
		ports = append(ports, corev1.ContainerPort{ContainerPort: int32(port)})
	}

	return corev1.Container{
		Name:            "pmm-client",
		Image:           pmmSpec.Image,
		ImagePullPolicy: pmmSpec.ImagePullPolicy,
		SecurityContext: pmmSpec.ContainerSecurityContext,
		Ports:           ports,
		Resources:       pmmSpec.Resources,
		Env: []corev1.EnvVar{
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name:  "CLUSTER_NAME",
				Value: clusterName,
			},
			{
				Name:  "CLIENT_PORT_LISTEN",
				Value: "7777",
			},
			{
				Name:  "CLIENT_PORT_MIN",
				Value: "30100",
			},
			{
				Name:  "CLIENT_PORT_MAX",
				Value: "30105",
			},
			{
				Name:  "PMM_AGENT_SERVER_ADDRESS",
				Value: pmmSpec.ServerHost,
			},
			{
				Name:  "PMM_AGENT_SERVER_USERNAME",
				Value: pmmSpec.ServerUser,
			},
			{
				Name: "PMM_AGENT_SERVER_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: k8s.SecretKeySelector(secretsName, "pmmserver"),
				},
			},
			{
				Name:  "PMM_SERVER",
				Value: pmmSpec.ServerHost,
			},
			{
				Name:  "PMM_USER",
				Value: pmmSpec.ServerUser,
			},
			{
				Name: "PMM_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: k8s.SecretKeySelector(secretsName, "pmmserver"),
				},
			},
			{
				Name:  "PMM_AGENT_LISTEN_PORT",
				Value: "7777",
			},
			{
				Name:  "PMM_AGENT_PORTS_MIN",
				Value: "30100",
			},
			{
				Name:  "PMM_AGENT_PORTS_MAX",
				Value: "30105",
			},
			{
				Name:  "PMM_AGENT_CONFIG_FILE",
				Value: "/usr/local/percona/pmm2/config/pmm-agent.yaml",
			},
			{
				Name:  "PMM_AGENT_SERVER_INSECURE_TLS",
				Value: "1",
			},
			{
				Name:  "PMM_AGENT_LISTEN_ADDRESS",
				Value: "0.0.0.0",
			},
			{
				Name:  "PMM_AGENT_SETUP_NODE_NAME",
				Value: "$(POD_NAMESPACE)-$(POD_NAME)",
			},
			{
				Name:  "PMM_AGENT_SETUP_METRICS_MODE",
				Value: "push",
			},
			{
				Name:  "PMM_AGENT_SETUP",
				Value: "1",
			},
			{
				Name:  "PMM_AGENT_SETUP_FORCE",
				Value: "1",
			},
			{
				Name:  "PMM_AGENT_SETUP_NODE_TYPE",
				Value: "container",
			},
			{
				Name:  "PMM_AGENT_PRERUN_SCRIPT",
				Value: "pmm-admin status --wait=10s;\npmm-admin add ${DB_TYPE} ${PMM_ADMIN_CUSTOM_PARAMS} --skip-connection-check --metrics-mode=${PMM_AGENT_SETUP_METRICS_MODE} --username=${DB_USER} --password=${DB_PASSWORD} --cluster=${CLUSTER_NAME} --service-name=${PMM_AGENT_SETUP_NODE_NAME} --host=${POD_NAME} --port=${DB_PORT} ${DB_ARGS};\npmm-admin annotate --service-name=${PMM_AGENT_SETUP_NODE_NAME} 'Service restarted'",
			},
			{
				Name:  "PMM_AGENT_SIDECAR",
				Value: "true",
			},
			{
				Name:  "PMM_AGENT_SIDECAR_SLEEP",
				Value: "5",
			},
			{
				Name:  "DB_CLUSTER",
				Value: clusterName,
			},
			{
				Name:  "DB_TYPE",
				Value: componentName,
			},
			{
				Name:  "DB_HOST",
				Value: "localhost",
			},
			{
				Name:  "DB_PORT",
				Value: "33062",
			},
			{
				Name:  "DB_USER",
				Value: string(apiv1alpha1.UserMonitor),
			},
			{
				Name: "DB_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: k8s.SecretKeySelector(secretsName, string(apiv1alpha1.UserMonitor)),
				},
			},
			{
				Name:  "DB_ARGS",
				Value: "--query-source=perfschema",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      dataVolumeName,
				MountPath: DataMountPath,
			},
		},
	}
}

func appendUniqueContainers(containers []corev1.Container, more ...corev1.Container) []corev1.Container {
	if len(more) == 0 {
		return containers
	}

	exists := make(map[string]bool)
	for i := range containers {
		exists[containers[i].Name] = true
	}

	for i := range more {
		name := more[i].Name
		if exists[name] {
			continue
		}

		containers = append(containers, more[i])
		exists[name] = true
	}

	return containers
}
