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
	corev1 "k8s.io/api/core/v1"
	beta1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"time"

	webappv1 "kubebuilder_test/elasticweb-operator/api/v1"
)

// NodePoolReconciler reconciles a NodePool object
type NodePoolReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

const nodeFinalizer = "node.finalizers.node-pool.webapp.elasticweb-operator"

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
	logg.Info(fmt.Sprintf("4. nodeLabelSelector is : [%s] ", labelSelector.String()))

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

	logg.Info(fmt.Sprintf("6. Finalizers info : [%v]", pool.Finalizers))
	// 进入预删除流程
	if !pool.DeletionTimestamp.IsZero() {
		logg.Info("6.1 start delete Finalizer work ")
		return ctrl.Result{}, r.nodeFinalizer(ctx, pool, nodes.Items)
	}

	// 如果删除时间戳为空说明现在不需要删除该数据，我们将 nodeFinalizer 加入到资源中
	if !containsString(pool.Finalizers, nodeFinalizer) {
		logg.Info("6.2 add  Finalizer")
		pool.Finalizers = append(pool.Finalizers, nodeFinalizer)
		if err := r.Client.Update(ctx, pool); err != nil {
			logg.Info("6.3 add  Finalizer error")
			return ctrl.Result{}, err
		}
	}

	logg.Info("7. if have nodes, will merge data", "nodes", len(nodes.Items))
	if len(nodes.Items) > 0 {

		// update status
		logg.Info("7.1 update pools status")
		pool.Status.Allocatable = corev1.ResourceList{}
		pool.Status.NodeCount = len(nodes.Items)

		logg.Info("7.2 start patch node")
		for _, node := range nodes.Items {
			node := node
			if err := r.Patch(ctx, pool.Spec.ApplyNode(node), client.Merge); err != nil {
				logg.Error(err, "7.3 patch node error")
				return ctrl.Result{}, nil
			}
			for name, quantity := range node.Status.Allocatable {
				q, ok := pool.Status.Allocatable[name]
				if ok {
					// 存在就更新 value
					q.Add(quantity)
					pool.Status.Allocatable[name] = q
					continue
				}
				// 不存在就添加
				pool.Status.Allocatable[name] = quantity
			}
		}
	}

	runtimeClass := &beta1.RuntimeClass{}
	logg.Info("8. get runtimeClass info")
	if err := r.Get(ctx, client.ObjectKeyFromObject(pool.RuntimeClass()), runtimeClass); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logg.Error(err, "8.1 get runtimeClass error")
			return ctrl.Result{}, err
		}

		logg.Info("8.2 runtimeClass not found， maybe create new one")
	}

	// 如果不存在创建一个新的
	logg.Info("9. runtimeClass is not exists, create new one")
	if runtimeClass.Name == "" {
		runtimeClass := pool.RuntimeClass()
		err := ctrl.SetControllerReference(pool, runtimeClass, r.Scheme)
		if err != nil {
			logg.Error(err, "9.1 set controller reference error")
		}
		err = r.Create(ctx, runtimeClass)
		if err != nil {
			logg.Error(err, "9.2 create runtimeClass error")
			return ctrl.Result{}, nil
		}
		logg.Info("9.3 runtimeClass create success")
	}

	logg.Info("10. patch runtimeClass info")
	if err := r.Client.Patch(ctx, pool.RuntimeClass(), client.Merge); err != nil {
		logg.Error(err, "10.1 patch runtimeClass error")
		return ctrl.Result{}, nil
	}

	r.Recorder.Event(pool, corev1.EventTypeNormal, "test", "test")

	pool.Status.Status = 200
	logg.Info("11. update nodePool status fields")
	err := r.Status().Update(ctx, pool)
	if err != nil {
		logg.Error(err, "11.1 update nodePool status fields error")
		return ctrl.Result{}, err
	} else {
		logg.Info("11.2 update nodePool status fields success")
	}

	logg.Info("12. reconcile success")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// watches 增加监测 Node 更新事件
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.NodePool{}).
		Watches(&source.Kind{Type: &corev1.Node{}}, handler.Funcs{UpdateFunc: r.nodeUpdateHandler}).
		Complete(r)
}

func (r *NodePoolReconciler) nodeUpdateHandler(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	logg := log.FromContext(ctx)
	defer cancel()

	oldPool, err := r.getNodePoolByLabels(ctx, e.ObjectOld.GetLabels())
	if err != nil {
		logg.Error(err, "get node pool err")
	}

	if oldPool != nil {
		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{Name: oldPool.Name},
		})
	}

	newPool, err := r.getNodePoolByLabels(ctx, e.ObjectNew.GetLabels())
	if err != nil {
		logg.Error(err, "get node pool err")
	}
	if newPool != nil {
		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{Name: newPool.Name},
		})
	}

}

func (r *NodePoolReconciler) getNodePoolByLabels(ctx context.Context, labels map[string]string) (*webappv1.NodePool, error) {
	pool := &webappv1.NodePool{}

	for k := range labels {
		ss := strings.Split(k, "node-role.kubernetes.io/")
		if len(ss) != 2 {
			continue
		}

		err := r.Client.Get(ctx, types.NamespacedName{Name: ss[1]}, pool)
		if err == nil {
			return pool, nil
		}

		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
	}
	return nil, nil
}

// 节点预删除逻辑
func (r *NodePoolReconciler) nodeFinalizer(ctx context.Context, pool *webappv1.NodePool, nodes []corev1.Node) error {
	// 不为空就说明进入到预删除流程
	for _, n := range nodes {
		n := n

		// 更新节点的标签和污点信息
		err := r.Update(ctx, pool.Spec.CleanNode(n))
		if err != nil {
			return err
		}
	}

	// 预删除执行完毕，移除 nodeFinalizer
	pool.Finalizers = removeString(pool.Finalizers, nodeFinalizer)
	return r.Client.Update(ctx, pool)
}

// 辅助函数用于检查并从字符串切片中删除字符串。
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
