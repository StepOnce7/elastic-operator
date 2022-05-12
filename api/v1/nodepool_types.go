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

package v1

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	beta1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodePoolSpec defines the desired state of NodePool
type NodePoolSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// 污点
	Taints []v1.Taint `json:"taints,omitempty"`

	// 标签
	Labels map[string]string `json:"labels,omitempty"`

	Handler string `json:"handler,omitempty"`
}

func (spec *NodePoolSpec) CleanNode(node corev1.Node) *corev1.Node {
	nodeLabels := map[string]string{}

	for k, v := range node.Labels {
		if strings.Contains(k, "kubernetes") {
			nodeLabels[k] = v
		}
	}

	node.Labels = nodeLabels

	var taints []corev1.Taint
	for _, taint := range node.Spec.Taints {
		if strings.Contains(taint.Key, "kubernetes") {
			taints = append(taints, taint)
		}
	}

	node.Spec.Taints = taints

	return &node
}

// ApplyNode 生成 Node 结构，可以用于 Patch 数据
func (spec *NodePoolSpec) ApplyNode(node corev1.Node) *corev1.Node {
	n := spec.CleanNode(node)

	for k, v := range spec.Labels {
		n.Labels[k] = v
	}

	n.Spec.Taints = append(n.Spec.Taints, spec.Taints...)
	return n
}

// NodePoolStatus defines the observed state of NodePool
type NodePoolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status            int `json:"status"`
	NodeCount         int `json:"nodeCount"`
	NotReadyNodeCount int `json:"notReadyNodeCount"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NodePool is the Schema for the nodepools API
type NodePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodePoolSpec   `json:"spec,omitempty"`
	Status NodePoolStatus `json:"status,omitempty"`
}

func (in *NodePool) String() string {
	return fmt.Sprintf("Taints [%v], Labels [%v]",
		in.Spec.Taints,
		in.Spec.Labels)
}

// NodeRole 返回节点对应的 role 标签名
func (in *NodePool) NodeRole() string {
	return "node-role.kubernetes.io/" + in.Name
}

// NodeLabelSelector 返回节点 label 选择器
func (in *NodePool) NodeLabelSelector() labels.Selector {
	return labels.SelectorFromSet(map[string]string{
		in.NodeRole(): "",
	})
}

// RuntimeClass 生成对应的 runtime class 对象
func (in *NodePool) RuntimeClass() *beta1.RuntimeClass {
	s := in.Spec
	tolerations := make([]corev1.Toleration, len(s.Taints))
	for i, t := range s.Taints {
		tolerations[i] = corev1.Toleration{
			Key:      t.Key,
			Value:    t.Value,
			Effect:   t.Effect,
			Operator: corev1.TolerationOpEqual,
		}
	}

	// create a new runtimeClass by nodePool info
	return &beta1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-pool-" + in.Name,
		},
		Handler: "runc",
		Scheduling: &beta1.Scheduling{
			NodeSelector: s.Labels,
			Tolerations:  tolerations,
		},
	}
}

//+kubebuilder:object:root=true

// NodePoolList contains a list of NodePool
type NodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodePool{}, &NodePoolList{})
}
