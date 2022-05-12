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
	corev1 "k8s.io/api/core/v1"
	beta1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	webappv1 "kubebuilder_test/elasticweb-operator/api/v1"
)

// NodePoolReconciler reconciles a NodePool object
type NodePoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const nodeFinalizer = "node.finalizers.node-pool.lailin.xyz"

//+kubebuilder:rbac:groups=webapp.elasticweb-operator,resources=nodepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=webapp.elasticweb-operator,resources=nodepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=webapp.elasticweb-operator,resources=nodepools/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NodePool object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *NodePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logg := log.FromContext(ctx)
	logg.Info("1. start nodePool reconcile logic")

	pool := &webappv1.NodePool{}

	logg.Info("2. get nodePool instance")
	if err := r.Get(ctx, req.NamespacedName, pool); err != nil {
		if errors.IsNotFound(err) {
			logg.Info("2.1 nodePool instance not found, maybe removed")
			return reconcile.Result{}, nil
		}

		logg.Error(err, "2.2 get nodePool instance error")
		return ctrl.Result{}, err
	}

	logg.Info("3. nodePool instance : " + pool.String())

	labelSelector := pool.NodeLabelSelector()
	logg.WithValues("4. nodeLabelSelector is : ", labelSelector)

	var nodes corev1.NodeList
	logg.Info("5. get nodeList by labelSelector")
	// 查看是否存在对应的节点，如果存在那么就给这些节点加上数据
	if err := r.List(ctx, &nodes, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
		if errors.IsNotFound(err) {
			logg.Info("5.1 nodeList info not found, maybe removed")
			return reconcile.Result{}, nil
		}

		logg.Error(err, "5.2 get nodeList info error")
		return ctrl.Result{}, err
	}

	if len(nodes.Items) > 0 {
		logg.Info("5.1 find nodes, will merge data", "nodes", len(nodes.Items))
		for _, node := range nodes.Items {
			node := node
			if err := r.Patch(ctx, pool.Spec.ApplyNode(node), client.Merge); err != nil {
				logg.Error(err, "5.2 patch node error")
				return ctrl.Result{}, nil
			}
		}
	}

	runtimeClass := &beta1.RuntimeClass{}
	logg.Info("6. get runtimeClass info")
	if err := r.Get(ctx, client.ObjectKeyFromObject(pool.RuntimeClass()), runtimeClass); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logg.Error(err, "6.1 get runtimeClass error")
			return ctrl.Result{}, err
		}

		logg.Info("6.2 runtimeClass not found， maybe create new one")
	}

	// 如果不存在创建一个新的
	logg.Info("7 runtimeClass is not exists, create new one")
	if runtimeClass.Name == "" {
		err := r.Create(ctx, pool.RuntimeClass())
		if err != nil {
			logg.Error(err, "7.1 create runtimeClass error")
			return ctrl.Result{}, nil
		}
		logg.Info("7.2 runtimeClass create success")
	}

	logg.Info("8. update runtimeClass info")
	if err := r.Client.Patch(ctx, pool.RuntimeClass(), client.Merge); err != nil {
		logg.Error(err, "8.1 update runtimeClass error")
		return ctrl.Result{}, nil
	}

	logg.Info("9. reconcile success")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.NodePool{}).
		Complete(r)
}
