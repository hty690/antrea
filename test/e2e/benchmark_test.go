// Copyright 2021 Antrea Authors
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
	"fmt"
	"strconv"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
)

const (
	intraNode          = 0
	interNode          = 1
	roundNum           = 3
	netperfControlPort = 12865
	netperfDataPort1   = 10000
	netperfDataPort2   = 10001
	netperfDataPort3   = 10002
	iperfImage         = "networkstatic/iperf3"
	netperfImage       = "sirot/netperf-latest"
)

func TestAntreaBenchmark(t *testing.T) {
	skipIfNotIPv4Cluster(t)
	skipIfHasWindowsNodes(t)

	data, err := setupTest(t)
	if err != nil {
		t.Fatalf("Error when setting up test: %v", err)
	}
	defer teardownTest(t, data)

	if err := data.createPodOnNode("iperf-local-client", controlPlaneNodeName(), iperfImage, nil, nil, nil, nil, false, nil); err != nil {
		t.Fatalf("Error when creating the iperf client Pod: %v", err)
	}
	if err := data.podWaitForRunning(defaultTimeout, "iperf-local-client", testNamespace); err != nil {
		t.Fatalf("Error when waiting for the iperf client Pod: %v", err)
	}
	if err := data.createPodOnNode("iperf-local-server", controlPlaneNodeName(), iperfImage, nil, nil, nil, []v1.ContainerPort{
		{Protocol: v1.ProtocolTCP, ContainerPort: iperfPort},
		{Protocol: v1.ProtocolUDP, ContainerPort: iperfPort}}, false, nil); err != nil {
		t.Fatalf("Error when creating the iperf local server Pod: %v", err)
	}
	localSvrIPs, err := data.podWaitForIPs(defaultTimeout, "iperf-local-server", testNamespace)
	if err != nil {
		t.Fatalf("Error when waiting for the iperf local server Pod: %v", err)
	}
	if err := data.createPodOnNode("iperf-remote-server", workerNodeName(1), iperfImage, nil, nil, nil, []v1.ContainerPort{
		{Protocol: v1.ProtocolTCP, ContainerPort: iperfPort},
		{Protocol: v1.ProtocolUDP, ContainerPort: iperfPort}}, false, nil); err != nil {
		t.Fatalf("Error when creating the iperf remote server Pod: %v", err)
	}
	remoteSvrIPs, err := data.podWaitForIPs(defaultTimeout, "iperf-remote-server", testNamespace)
	if err != nil {
		t.Fatalf("Error when waiting for the iperf remote server Pod: %v", err)
	}

	localSvc, err := data.createService("iperf-local-server", iperfPort, iperfPort, map[string]string{"antrea-e2e": "iperf-local-server"}, false, v1.ServiceTypeClusterIP, nil)
	if err != nil {
		t.Fatalf("Error when creating iperf-local-server service: %v", err)
	}
	remoteSvc, err := data.createService("iperf-remote-server", iperfPort, iperfPort, map[string]string{"antrea-e2e": "iperf-remote-server"}, false, v1.ServiceTypeClusterIP, nil)
	if err != nil {
		t.Fatalf("Error when creating iperf-remote-server service: %v", err)
	}

	t.Run("testIperfIntraNode", func(t *testing.T) { testIperf(t, data, localSvrIPs, localSvc, intraNode) })
	t.Run("testIperfInterNode", func(t *testing.T) { testIperf(t, data, remoteSvrIPs, remoteSvc, interNode) })
}

func testIperf(t *testing.T, data *TestData, podIPs *PodIPs, svc *v1.Service, testType int) {
	runIperf := func(cmd string) (bandwidth float64) {
		stdout, _, err := data.runCommandFromPod(testNamespace, "iperf-local-client", "iperf3", []string{"bash", "-c", cmd})
		if err != nil {
			t.Fatalf("Error when running iperf3 client: %v", err)
		}
		bandwidth, err = strconv.ParseFloat(strings.TrimSpace(stdout), 64)
		if err != nil {
			t.Errorf("Error parsing string to float64: %v", err)
		}
		return
	}

	var connType string
	if testType == intraNode {
		connType = "Intra"
	} else if testType == interNode {
		connType = "Inter"
	} else {
		t.Fatal("Get wrong test type")
	}
	cmd := fmt.Sprintf("iperf3 -u -b 0 -f m -w 256K -O 1 -c %s | grep receiver | awk '{print $7}'", podIPs.ipv4.String())
	var acc float64
	for i := 0; i < roundNum; i++ {
		acc += runIperf(cmd)
	}
	t.Logf("%s node pod to pod UDP bandwidth: %v Mbits/sec", connType, acc/roundNum)

	cmd = fmt.Sprintf("iperf3 -u -b 0 -f m -w 256K -O 1 -c %s | grep receiver | awk '{print $7}'", svc.Spec.ClusterIP)
	acc = 0
	for i := 0; i < roundNum; i++ {
		acc += runIperf(cmd)
	}
	t.Logf("%s node pod to svc UDP bandwidth: %v Mbits/sec", connType, acc/roundNum)
}
