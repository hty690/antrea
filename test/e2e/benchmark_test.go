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
	heyImage           = "ricoli/hey"
)

func TestAntreaBenchmark(t *testing.T) {
	skipIfNotIPv4Cluster(t)
	skipIfHasWindowsNodes(t)

	data, err := setupTest(t)
	if err != nil {
		t.Fatalf("Error when setting up test: %v", err)
	}
	defer teardownTest(t, data)

	ipFamily := v1.IPv4Protocol
	localSvc, err := data.createMultiPortService("iperf-local-server", map[int32]int32{iperfPort: iperfPort},
		map[string]string{"antrea-e2e": "iperf-local-server"}, false, v1.ServiceTypeClusterIP, &ipFamily, []v1.Protocol{v1.ProtocolTCP, v1.ProtocolUDP})
	if err != nil {
		t.Fatalf("Error when creating iperf-local-server service: %v", err)
	}
	remoteSvc, err := data.createMultiPortService("iperf-remote-server", map[int32]int32{iperfPort: iperfPort},
		map[string]string{"antrea-e2e": "iperf-remote-server"}, false, v1.ServiceTypeClusterIP, &ipFamily, []v1.Protocol{v1.ProtocolTCP, v1.ProtocolUDP})
	if err != nil {
		t.Fatalf("Error when creating iperf-remote-server service: %v", err)
	}

	iperfCmd := []string{"iperf3", "-s"}
	if err := data.createPodOnNode("iperf-local-client", controlPlaneNodeName(), iperfImage, iperfCmd, nil, nil, nil, false, nil); err != nil {
		t.Fatalf("Error when creating the iperf client Pod: %v", err)
	}
	if err := data.podWaitForRunning(defaultTimeout, "iperf-local-client", testNamespace); err != nil {
		t.Fatalf("Error when waiting for the iperf client Pod: %v", err)
	}
	if err := data.createPodOnNode("iperf-local-server", controlPlaneNodeName(), iperfImage, iperfCmd, nil, nil, []v1.ContainerPort{
		{Protocol: v1.ProtocolTCP, ContainerPort: iperfPort},
		{Protocol: v1.ProtocolUDP, ContainerPort: iperfPort}}, false, nil); err != nil {
		t.Fatalf("Error when creating the iperf local server Pod: %v", err)
	}
	localSvrIPs, err := data.podWaitForIPs(defaultTimeout, "iperf-local-server", testNamespace)
	if err != nil {
		t.Fatalf("Error when waiting for the iperf local server Pod: %v", err)
	}
	if err := data.createPodOnNode("iperf-remote-server", workerNodeName(1), iperfImage, iperfCmd, nil, nil, []v1.ContainerPort{
		{Protocol: v1.ProtocolTCP, ContainerPort: iperfPort},
		{Protocol: v1.ProtocolUDP, ContainerPort: iperfPort}}, false, nil); err != nil {
		t.Fatalf("Error when creating the iperf remote server Pod: %v", err)
	}
	remoteSvrIPs, err := data.podWaitForIPs(defaultTimeout, "iperf-remote-server", testNamespace)
	if err != nil {
		t.Fatalf("Error when waiting for the iperf remote server Pod: %v", err)
	}

	t.Run("testIperfIntraNode", func(t *testing.T) { testIperf(t, data, localSvrIPs, localSvc, intraNode) })
	t.Run("testIperfInterNode", func(t *testing.T) { testIperf(t, data, remoteSvrIPs, remoteSvc, interNode) })
}

func TestNetperfBenchmark(t *testing.T) {
	skipIfNotIPv4Cluster(t)
	skipIfHasWindowsNodes(t)

	data, err := setupTest(t)
	if err != nil {
		t.Fatalf("Error when setting up test: %v", err)
	}
	defer teardownTest(t, data)

	ipFamily := v1.IPv4Protocol
	netperfPorts := map[int32]int32{
		netperfControlPort: netperfControlPort,
		netperfDataPort1:   netperfDataPort1,
		netperfDataPort2:   netperfDataPort2,
		netperfDataPort3:   netperfDataPort3,
	}
	localSvc, err := data.createMultiPortService("netperf-local-server", netperfPorts, map[string]string{"antrea-e2e": "netperf-local-server"},
		false, v1.ServiceTypeClusterIP, &ipFamily, []v1.Protocol{v1.ProtocolTCP, v1.ProtocolUDP})
	if err != nil {
		t.Fatalf("Error when creating netperf-local-server service: %v", err)
	}
	remoteSvc, err := data.createMultiPortService("netperf-remote-server", netperfPorts, map[string]string{"antrea-e2e": "netperf-remote-server"},
		false, v1.ServiceTypeClusterIP, &ipFamily, []v1.Protocol{v1.ProtocolTCP, v1.ProtocolUDP})
	if err != nil {
		t.Fatalf("Error when creating netperf-remote-server service: %v", err)
	}

	netperfCmd := []string{"netserver", "-D"}
	if err := data.createPodOnNode("netperf-local-client", controlPlaneNodeName(), netperfImage, netperfCmd, nil, nil, nil, false, nil); err != nil {
		t.Fatalf("Error when creating the netperf client Pod: %v", err)
	}
	if err := data.podWaitForRunning(defaultTimeout, "netperf-local-client", testNamespace); err != nil {
		t.Fatalf("Error when waiting for the netperf client Pod: %v", err)
	}
	if err := data.createPodOnNode("netperf-local-server", controlPlaneNodeName(), netperfImage, netperfCmd, nil, nil, nil, false, nil); err != nil {
		t.Fatalf("Error when creating the netperf local server Pod: %v", err)
	}
	localSvrIPs, err := data.podWaitForIPs(defaultTimeout, "netperf-local-server", testNamespace)
	if err != nil {
		t.Fatalf("Error when waiting for the netperf local server Pod: %v", err)
	}
	if err := data.createPodOnNode("netperf-remote-server", workerNodeName(1), netperfImage, netperfCmd, nil, nil, nil, false, nil); err != nil {
		t.Fatalf("Error when creating the netperf remote server Pod: %v", err)
	}
	remoteSvrIPs, err := data.podWaitForIPs(defaultTimeout, "netperf-remote-server", testNamespace)
	if err != nil {
		t.Fatalf("Error when waiting for the netperf remote server Pod: %v", err)
	}

	t.Run("testNetperfTCPBandwidth, IntraNode", func(t *testing.T) { testNetperfTCPBandwidth(t, data, localSvrIPs, localSvc, intraNode) })
	t.Run("testNetperfTCPBandwidth, InterNode", func(t *testing.T) { testNetperfTCPBandwidth(t, data, remoteSvrIPs, remoteSvc, interNode) })
	t.Run("testNetperfTCPRR, IntraNode", func(t *testing.T) { testNetperfTCPRR(t, data, remoteSvrIPs, remoteSvc, intraNode) })
	t.Run("testNetperfTCPRR, InterNode", func(t *testing.T) { testNetperfTCPRR(t, data, remoteSvrIPs, remoteSvc, interNode) })
	t.Run("testNetperfTCPConnRR, IntraNode", func(t *testing.T) { testNetperfTCPConnRR(t, data, remoteSvrIPs, remoteSvc, intraNode) })
	t.Run("testNetperfTCPConnRR, InterNode", func(t *testing.T) { testNetperfTCPConnRR(t, data, remoteSvrIPs, remoteSvc, interNode) })

}

func testIperf(t *testing.T, data *TestData, podIPs *PodIPs, svc *v1.Service, testType int) {
	runIperf := func(cmd string) (bandwidth float64) {
		stdout, stderr, err := data.runCommandFromPod(testNamespace, "iperf-local-client", "iperf3", []string{"bash", "-c", cmd})
		if err != nil {
			t.Fatalf("Error when running iperf3 client: %v", err)
		}
		if stderr != "" {
			t.Errorf("Wrong results from iperf3 client: %s", stderr)
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
	t.Logf("%s node pod to pod UDP bandwidth: %.2f Mbits/sec", connType, acc/roundNum)

	cmd = fmt.Sprintf("iperf3 -u -b 0 -f m -w 256K -O 1 -c %s | grep receiver | awk '{print $7}'", svc.Spec.ClusterIP)
	acc = 0
	for i := 0; i < roundNum; i++ {
		acc += runIperf(cmd)
	}
	t.Logf("%s node pod to svc UDP bandwidth: %.2f Mbits/sec", connType, acc/roundNum)
}

func runNetperf(t *testing.T, data *TestData, cmd string) (r float64) {
	stdout, stderr, err := data.runCommandFromPod(testNamespace, "netperf-local-client", "netperf-latest", []string{"bash", "-c", cmd})
	if err != nil || stderr != "" {
		t.Errorf("Error when running netperf client: %v", err)
		t.Fatalf("Wrong results from netperf client: %s", stderr)
	}
	r, err = strconv.ParseFloat(strings.TrimSpace(stdout), 64)
	if err != nil {
		t.Errorf("Error parsing string to float64: %v", err)
	}
	return
}

func testNetperfTCPBandwidth(t *testing.T, data *TestData, podIPs *PodIPs, svc *v1.Service, testType int) {
	var connType string
	if testType == intraNode {
		connType = "Intra"
	} else if testType == interNode {
		connType = "Inter"
	} else {
		t.Fatal("Get wrong test type")
	}
	var (
		cmd string
		acc float64 = 0
	)
	for i := 10000; i <= 10002; i++ {
		cmd = fmt.Sprintf("netperf -H %s -t TCP_STREAM -- -P %s|grep '16384'|awk '{print $5}'", podIPs.ipv4.String(), strconv.Itoa(i))
		acc += runNetperf(t, data, cmd)
	}
	t.Logf("%s node pod to pod TCP bandwidth: %.2f Mbits/sec", connType, acc/roundNum)

	acc = 0
	for i := 10000; i <= 10002; i++ {
		cmd = fmt.Sprintf("netperf -H %s -t TCP_STREAM -- -P %s|grep '16384'|awk '{print $5}'", svc.Spec.ClusterIP, strconv.Itoa(i))
		acc += runNetperf(t, data, cmd)
	}
	t.Logf("%s node pod to svc TCP bandwidth: %.2f Mbits/sec", connType, acc/roundNum)
}

func testNetperfTCPRR(t *testing.T, data *TestData, podIPs *PodIPs, svc *v1.Service, testType int) {
	var connType string
	if testType == intraNode {
		connType = "Intra"
	} else if testType == interNode {
		connType = "Inter"
	} else {
		t.Fatal("Get wrong test type")
	}
	var (
		cmd string
		acc float64 = 0
	)
	for i := 10000; i <= 10002; i++ {
		cmd = fmt.Sprintf("netperf -H %s -t TCP_RR -- -P %s|grep '131072 1'|awk '{print $6}'", podIPs.ipv4.String(), strconv.Itoa(i))
		acc += runNetperf(t, data, cmd)
	}
	t.Logf("%s node pod to pod TCP_RR: %.2f trans/sec", connType, acc/roundNum)

	acc = 0
	for i := 10000; i <= 10002; i++ {
		cmd = fmt.Sprintf("netperf -H %s -t TCP_RR -- -P %s|grep '131072 1'|awk '{print $6}'", svc.Spec.ClusterIP, strconv.Itoa(i))
		acc += runNetperf(t, data, cmd)
	}
	t.Logf("%s node pod to svc TCP_RR: %.2f trans/sec", connType, acc/roundNum)
}

func testNetperfTCPConnRR(t *testing.T, data *TestData, podIPs *PodIPs, svc *v1.Service, testType int) {
	var connType string
	if testType == intraNode {
		connType = "Intra"
	} else if testType == interNode {
		connType = "Inter"
	} else {
		t.Fatal("Get wrong test type")
	}
	var (
		cmd string
		acc float64 = 0
	)
	for i := 10000; i <= 10002; i++ {
		// Sleep 120s to wait for flush of conntrack table and ports
		// t.Log("Sleeping 120 seconds")
		// time.Sleep(2 * time.Minute)
		cmd = fmt.Sprintf("netperf -H %s -t TCP_CRR -- -P %s|grep '131072 1'|awk '{print $6}'", podIPs.ipv4.String(), strconv.Itoa(i))
		acc += runNetperf(t, data, cmd)
	}
	t.Logf("%s node pod to pod TCP_CRR: %.2f trans/sec", connType, acc/roundNum)

	acc = 0
	for i := 10000; i <= 10002; i++ {
		// t.Log("Sleeping 120 seconds")
		// time.Sleep(2 * time.Minute)
		cmd = fmt.Sprintf("netperf -H %s -t TCP_CRR -- -P %s|grep '131072 1'|awk '{print $6}'", svc.Spec.ClusterIP, strconv.Itoa(i))
		acc += runNetperf(t, data, cmd)
	}
	t.Logf("%s node pod to svc TCP_CRR: %.2f trans/sec", connType, acc/roundNum)
}

func TestNginxRPS(t *testing.T) {
	skipIfNotIPv4Cluster(t)
	skipIfHasWindowsNodes(t)

	data, err := setupTest(t)
	if err != nil {
		t.Fatalf("Error when setting up test: %v", err)
	}
	defer teardownTest(t, data)

	ipFamily := v1.IPv4Protocol

	localSvc, err := data.createService("nginx-local-server", 80, 80, map[string]string{"antrea-e2e": "nginx-local-server"},
		false, v1.ServiceTypeClusterIP, &ipFamily)
	if err != nil {
		t.Fatalf("Error when creating nginx-local-server service: %v", err)
	}
	remoteSvc, err := data.createService("nginx-remote-server", 80, 80, map[string]string{"antrea-e2e": "nginx-remote-server"},
		false, v1.ServiceTypeClusterIP, &ipFamily)
	if err != nil {
		t.Fatalf("Error when creating nginx-remote-server service: %v", err)
	}

	if err := data.createPodOnNode("nginx-local-client", controlPlaneNodeName(), heyImage, []string{"sleep", "7d"}, nil, nil, nil, false, nil); err != nil {
		t.Fatalf("Error when creating the nginx client Pod: %v", err)
	}
	if err := data.podWaitForRunning(defaultTimeout, "nginx-local-client", testNamespace); err != nil {
		t.Fatalf("Error when waiting for the nginx client Pod: %v", err)
	}
	if err := data.createPodOnNode("nginx-local-server", controlPlaneNodeName(), nginxImage, nil, nil, nil, nil, false, nil); err != nil {
		t.Fatalf("Error when creating the nginx remote server Pod: %v", err)
	}
	localSvrIPs, err := data.podWaitForIPs(defaultTimeout, "nginx-local-server", testNamespace)
	if err != nil {
		t.Fatalf("Error when waiting for the nginx local server Pod: %v", err)
	}
	if err := data.createPodOnNode("nginx-remote-server", workerNodeName(1), nginxImage, nil, nil, nil, nil, false, nil); err != nil {
		t.Fatalf("Error when creating the nginx remote server Pod: %v", err)
	}
	remoteSvrIPs, err := data.podWaitForIPs(defaultTimeout, "nginx-remote-server", testNamespace)
	if err != nil {
		t.Fatalf("Error when waiting for the nginx remote server Pod: %v", err)
	}

	t.Run("testNginxRPS, IntraNode", func(t *testing.T) { testNginxRPS(t, data, localSvrIPs, localSvc, intraNode) })
	t.Run("testNginxRPS, InterNode", func(t *testing.T) { testNginxRPS(t, data, remoteSvrIPs, remoteSvc, interNode) })

}

func testNginxRPS(t *testing.T, data *TestData, podIPs *PodIPs, svc *v1.Service, testType int) {
	runHey := func(cmd string) (rps float64) {
		stdout, stderr, err := data.runCommandFromPod(testNamespace, "nginx-local-client", "hey", []string{"sh", "-c", cmd})
		if err != nil {
			t.Fatalf("Error when running nginx client: %v", err)
		}
		rps, err = strconv.ParseFloat(strings.TrimSpace(stdout), 64)
		if err != nil {
			t.Errorf("Error parsing string to float64: %v", err)
			t.Errorf("STDERR: %s", stderr)
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
	var (
		cmd string
		acc float64 = 0
	)
	for i := 0; i < roundNum; i++ {
		cmd = fmt.Sprintf("hey -c 1 -n 5000 -disable-keepalive  http://%s  | grep Requests/sec: |awk '{print $2}'", podIPs.ipv4.String())
		acc += runHey(cmd)
	}
	t.Logf("%s node pod to pod hey test: %.2f reqs/sec", connType, acc/roundNum)

	acc = 0
	for i := 10000; i <= 10002; i++ {
		cmd = fmt.Sprintf("hey -c 1 -n 5000 -disable-keepalive  http://%s  | grep Requests/sec: |awk '{print $2}'", svc.Spec.ClusterIP)
		acc += runHey(cmd)
	}
	t.Logf("%s node pod to svc hey test: %.2f reqs/sec", connType, acc/roundNum)
}
