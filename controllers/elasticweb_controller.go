/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	webappv1 "kubebuilder_test/elasticweb-operator/api/v1"
)

const (
	APP_NAME       = "elastic-app"
	CONTAINER_PORT = 8080
	CPU_REQUEST    = "100m"
	CPU_LIMIT      = "100m"
	MEM_REQUEST    = "512Mi"
	MEM_LIMIT      = "512Mi"
)

// ElasticWebReconciler reconciles a ElasticWeb object
type ElasticWebReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=webapp.elasticweb-operator,resources=elasticwebs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=webapp.elasticweb-operator,resources=elasticwebs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=webapp.elasticweb-operator,resources=elasticwebs/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ElasticWeb object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ElasticWebReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logg := log.FromContext(ctx)
	logg.WithValues("elasticWeb", req.NamespacedName)

	logg.Info("1. start elasticWeb reconcile logic")

	instance := &webappv1.ElasticWeb{}

	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			logg.Info("2.1 instance not found, maybe removed")
			return reconcile.Result{}, nil
		}

		logg.Error(err, "2.2 error")
		return ctrl.Result{}, err
	}

	logg.Info("3. elasticWeb instance : " + instance.String())

	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, req.NamespacedName, deployment)

	if err != nil {
		if errors.IsNotFound(err) {
			logg.Info("4. deployment not exists")

			if *(instance.Spec.TotalQPS) < 1 {
				logg.Info("5.1 not need deployment")
				return ctrl.Result{}, nil
			}

			if err = createServiceIfNotExists(ctx, r, instance, req); err != nil {
				logg.Error(err, "5.2 error ")

				return ctrl.Result{}, err
			}

			if err = createDeployment(ctx, r, instance); err != nil {
				logg.Error(err, "5.3 error")
				return ctrl.Result{}, err
			}

			if err = updateStatus(ctx, r, instance); err != nil {
				logg.Error(err, "5.4 error")
				return ctrl.Result{}, nil
			}

			return ctrl.Result{}, nil
		} else {
			logg.Error(err, "7. error")
			return ctrl.Result{}, err
		}
	}

	expectReplicas := getExpectReplicas(instance)

	realReplicas := *deployment.Spec.Replicas
	logg.Info(fmt.Sprintf("9. expectReplicas [%d], realReplicas [%d]", expectReplicas, realReplicas))
	if expectReplicas == realReplicas {
		logg.Info("10. return now")
		return ctrl.Result{}, nil
	}

	// 如果不同,需要调整
	*(deployment.Spec.Replicas) = expectReplicas
	logg.Info("11. update deployment`s Replicas")

	if err = r.Update(ctx, deployment); err != nil {
		logg.Error(err, "12. update deployment replicas error")
		return ctrl.Result{}, err
	}

	logg.Info("13. update status")

	if err = updateStatus(ctx, r, instance); err != nil {
		logg.Error(err, "14. update status error")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ElasticWebReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.ElasticWeb{}).
		Complete(r)
}
func getExpectReplicas(elasticWeb *webappv1.ElasticWeb) int32 {
	singlePodQPS := *(elasticWeb.Spec.SinglePodQPS)
	totalQPS := *(elasticWeb.Spec.TotalQPS)

	replicas := totalQPS / singlePodQPS

	if totalQPS%singlePodQPS > 0 {
		replicas++
	}

	return replicas
}

func createServiceIfNotExists(ctx context.Context, r *ElasticWebReconciler, elasticWeb *webappv1.ElasticWeb, req ctrl.Request) error {
	logg := log.FromContext(ctx)

	service := &corev1.Service{}

	err := r.Get(ctx, req.NamespacedName, service)
	if err == nil {
		logg.Info("service exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, "query service error")
		return err
	}

	// instance a services
	service = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: elasticWeb.Namespace,
			Name:      elasticWeb.Name,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:     "http",
				Port:     8080,
				NodePort: *elasticWeb.Spec.Port,
			},
			},
			Selector: map[string]string{
				"app": APP_NAME,
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}

	logg.Info("set reference")

	// 关联
	if err := controllerutil.SetControllerReference(elasticWeb, service, r.Scheme); err != nil {
		logg.Error(err, "SetControllerReference error")
		return err
	}

	logg.Info("Start create service")
	if err := r.Create(ctx, service); err != nil {
		logg.Error(err, "create service error")
		return err
	}

	logg.Info("create service success")
	return nil
}

func createDeployment(ctx context.Context, r *ElasticWebReconciler, elasticWeb *webappv1.ElasticWeb) error {
	logg := log.FromContext(ctx)
	logg.WithValues("func", "createDeployment")

	expectReplicas := getExpectReplicas(elasticWeb)
	logg.Info(fmt.Sprintf("expectReplicas [%d]", expectReplicas))

	// instance
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: elasticWeb.Namespace,
			Name:      elasticWeb.Name,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(expectReplicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": APP_NAME,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": APP_NAME,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            APP_NAME,
							Image:           elasticWeb.Spec.Image,
							ImagePullPolicy: "IfNotPresent",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									Protocol:      corev1.ProtocolSCTP,
									ContainerPort: CONTAINER_PORT,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse(CPU_REQUEST),
									"memory": resource.MustParse(MEM_REQUEST),
								},
								Limits: corev1.ResourceList{
									"cpu":    resource.MustParse(CPU_LIMIT),
									"memory": resource.MustParse(MEM_LIMIT),
								},
							},
						},
					},
				},
			},
		},
	}

	logg.Info("set deployment reference")
	if err := controllerutil.SetControllerReference(elasticWeb, deployment, r.Scheme); err != nil {
		logg.Error(err, "SetControllerReference error")
		return err
	}

	logg.Info("start create deployment")
	if err := r.Create(ctx, deployment); err != nil {
		logg.Error(err, "create deployment error")
		return err
	}

	logg.Info("create deployment success")
	return nil
}

func updateStatus(ctx context.Context, r *ElasticWebReconciler, elasticWeb *webappv1.ElasticWeb) error {
	logg := log.FromContext(ctx)
	logg.WithValues("func", "updateStatus")

	singlePodQPS := *(elasticWeb.Spec.SinglePodQPS)

	replicas := getExpectReplicas(elasticWeb)

	if elasticWeb.Status.RealQPS == nil {
		elasticWeb.Status.RealQPS = new(int32)
	}

	*(elasticWeb.Status.RealQPS) = singlePodQPS * replicas

	logg.Info(fmt.Sprintf("singlePodQPS [%d], replicas [%d], realQPS[%d]", singlePodQPS, replicas, *(elasticWeb.Status.RealQPS)))

	if err := r.Status().Update(ctx, elasticWeb); err != nil {
		logg.Error(err, "update instance error")
		return err
	}

	return nil
}
