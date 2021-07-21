// Copyright 2019 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"context"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (data *TestData) createMultiPortService(serviceName string, ports map[int32]int32, selector map[string]string, affinity bool,
	serviceType corev1.ServiceType, ipFamily *corev1.IPFamily, protocols []corev1.Protocol) (*corev1.Service, error) {
	var svcPorts []corev1.ServicePort
	for p, tp := range ports {
		for _, proto := range protocols {
			svcPorts = append(svcPorts, corev1.ServicePort{
				Name:       strconv.Itoa(int(tp)) + "-" + strings.ToLower(string(proto)),
				Port:       p,
				TargetPort: intstr.FromInt(int(tp)),
				Protocol:   proto,
			})
		}
	}

	annotation := make(map[string]string)
	affinityType := corev1.ServiceAffinityNone
	var ipFamilies []corev1.IPFamily
	if ipFamily != nil {
		ipFamilies = append(ipFamilies, *ipFamily)
	}
	if affinity {
		affinityType = corev1.ServiceAffinityClientIP
	}

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: testNamespace,
			Labels: map[string]string{
				"antrea-e2e": serviceName,
				"app":        serviceName,
			},
			Annotations: annotation,
		},
		Spec: corev1.ServiceSpec{
			SessionAffinity: affinityType,
			Ports:           svcPorts,
			Type:            serviceType,
			Selector:        selector,
			IPFamilies:      ipFamilies,
		},
	}
	return data.clientset.CoreV1().Services(testNamespace).Create(context.TODO(), &service, metav1.CreateOptions{})
}
