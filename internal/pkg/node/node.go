/*
Copyright 2020 Rafael Fernández López <ereslibre@ereslibre.es>

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

package node

import (
	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	clusterv1alpha1 "oneinfra.ereslibre.es/m/apis/cluster/v1alpha1"
	"oneinfra.ereslibre.es/m/internal/pkg/infra"
)

// Role defines the role of this node
type Role string

const (
	// ControlPlaneRole is the role used for a Control Plane instance
	ControlPlaneRole Role = "control-plane"
	// ControlPlaneIngressRole is the role used for Control Plane ingress
	ControlPlaneIngressRole Role = "control-plane-ingress"
)

// Node represents a Control Plane node
type Node struct {
	Name               string
	Role               Role
	HypervisorName     string
	ClusterName        string
	AllocatedHostPorts map[string]int
}

// NewNodeWithRandomHypervisor creates a node with a random hypervisor from the provided hypervisorList
func NewNodeWithRandomHypervisor(clusterName, nodeName string, role Role, hypervisorList infra.HypervisorList) (*Node, error) {
	hypervisor, err := hypervisorList.Sample()
	if err != nil {
		return nil, err
	}
	node := Node{
		Name:               nodeName,
		HypervisorName:     hypervisor.Name,
		ClusterName:        clusterName,
		Role:               role,
		AllocatedHostPorts: map[string]int{},
	}
	apiserverHostPort, err := hypervisor.RequestPort(clusterName, nodeName)
	if err != nil {
		return nil, err
	}
	node.AllocatedHostPorts["apiserver"] = apiserverHostPort
	return &node, nil
}

// NewNodeFromv1alpha1 returns a node based on a versioned node
func NewNodeFromv1alpha1(node *clusterv1alpha1.Node) (*Node, error) {
	res := Node{
		Name:           node.ObjectMeta.Name,
		HypervisorName: node.Spec.Hypervisor,
		ClusterName:    node.Spec.Cluster,
	}
	switch node.Spec.Role {
	case clusterv1alpha1.ControlPlaneRole:
		res.Role = ControlPlaneRole
	case clusterv1alpha1.ControlPlaneIngressRole:
		res.Role = ControlPlaneIngressRole
	}
	res.AllocatedHostPorts = map[string]int{}
	for _, hostPort := range node.Status.AllocatedHostPorts {
		res.AllocatedHostPorts[hostPort.Name] = hostPort.Port
	}
	return &res, nil
}

// Export exports the node to a versioned node
func (node *Node) Export() *clusterv1alpha1.Node {
	res := &clusterv1alpha1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name,
		},
		Spec: clusterv1alpha1.NodeSpec{
			Hypervisor: node.HypervisorName,
			Cluster:    node.ClusterName,
		},
	}
	switch node.Role {
	case ControlPlaneRole:
		res.Spec.Role = clusterv1alpha1.ControlPlaneRole
	case ControlPlaneIngressRole:
		res.Spec.Role = clusterv1alpha1.ControlPlaneIngressRole
	}
	res.Status.AllocatedHostPorts = []clusterv1alpha1.NodeHostPortAllocation{}
	for hostPortName, hostPort := range node.AllocatedHostPorts {
		res.Status.AllocatedHostPorts = append(
			res.Status.AllocatedHostPorts,
			clusterv1alpha1.NodeHostPortAllocation{
				Name: hostPortName,
				Port: hostPort,
			},
		)
	}
	return res
}

// Specs returns the versioned specs of this node
func (node *Node) Specs() (string, error) {
	scheme := runtime.NewScheme()
	if err := clusterv1alpha1.AddToScheme(scheme); err != nil {
		return "", err
	}
	info, _ := runtime.SerializerInfoForMediaType(serializer.NewCodecFactory(scheme).SupportedMediaTypes(), runtime.ContentTypeYAML)
	encoder := serializer.NewCodecFactory(scheme).EncoderForVersion(info.Serializer, clusterv1alpha1.GroupVersion)
	nodeObject := node.Export()
	if encodedNode, err := runtime.Encode(encoder, nodeObject); err == nil {
		return string(encodedNode), nil
	}
	return "", errors.Errorf("could not encode node %q", node.Name)
}
