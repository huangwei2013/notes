package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"istio.io/istio/pkg/test/env"
	"istio.io/pkg/log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

var (
	testEnv        *env.TestSetup
	initMutex      sync.Mutex
	initEnvoyMutex sync.Mutex

	envoyStarted = false
	// Use 'service3' and 'app3' for pilot local tests.

	localIP = "10.3.0.3"
)

const (
	// 10.10.0.0/24 is service CIDR range

	// 10.0.0.0/9 is instance CIDR range
	app3Ip    = "10.2.0.1"
	gatewayIP = "10.3.0.1"
	ingressIP = "10.3.0.2"
)

func startEnvoy() {
	initEnvoyMutex.Lock()
	defer initEnvoyMutex.Unlock()
	if envoyStarted {
		return
	}

	tmplB, err := ioutil.ReadFile(env.IstioSrc + "/tests/testdata/bootstrap_tmpl.json")
	if err != nil {
		log.Errora("Can't read bootstrap template", err)
	}
	testEnv.EnvoyTemplate = string(tmplB)
	testEnv.Dir = env.IstioSrc
	nodeID := sidecarID(app3Ip, "app3")
	testEnv.EnvoyParams = []string{"--service-cluster", "serviceCluster", "--service-node", nodeID}
	testEnv.EnvoyConfigOpt = map[string]interface{}{
		"NodeID":  nodeID,
		"BaseDir": env.IstioSrc + "/tests/testdata/local",
		// Same value used in the real template
		"meta_json_str": fmt.Sprintf(`"BASE": "%s", ISTIO_VERSION: 1.5.0`, env.IstioSrc+"/tests/testdata/local"),
	}

	if err := testEnv.SetUp(); err != nil {
		log.Errora("Failed to setup test: %v", err)
	}

	envoyStarted = true
}

func sidecarID(ip, deployment string) string {
	return fmt.Sprintf("sidecar~%s~%s-644fc65469-96dza.testns~testns.svc.cluster.local", ip, deployment)
}

func gatewayID(ip string) string { //nolint: unparam
	return fmt.Sprintf("router~%s~istio-gateway-644fc65469-96dzt.istio-system~istio-system.svc.cluster.local", ip)
}

// stats2map parses envoy stats.
func stats2map(stats []byte) map[string]int {
	s := struct {
		Stats []EnvoyStat `json:"stats"`
	}{}
	_ = json.Unmarshal(stats, &s)
	m := map[string]int{}
	for _, stat := range s.Stats {
		m[stat.Name] = stat.Value
	}
	return m
}

func localPilotTestEnv(
	initFunc func(*bootstrap.Server),
	additionalArgs ...func(*bootstrap.PilotArgs)) (*bootstrap.Server, util.TearDownFunc) { //nolint: unparam
	initMutex.Lock()
	defer initMutex.Unlock()

	additionalArgs = append(additionalArgs, func(args *bootstrap.PilotArgs) {
		args.Plugins = bootstrap.DefaultPlugins
	})
	server, tearDown := util.EnsureTestServer(additionalArgs...)
	testEnv = env.NewTestSetup(env.XDSTest, nil)
	testEnv.Ports().PilotGrpcPort = uint16(util.MockPilotGrpcPort)
	testEnv.Ports().PilotHTTPPort = uint16(util.MockPilotHTTPPort)
	testEnv.IstioSrc = env.IstioSrc
	testEnv.IstioOut = env.IstioOut

	localIP = getLocalIP()

	// Run the initialization function.
	initFunc(server)

	// Trigger a push, to initiate push context with contents of registry.
	server.EnvoyXdsServer.Push(&model.PushRequest{Full: true})

	// Wait till a push is propagated.
	time.Sleep(200 * time.Millisecond)

	// Add a dummy client connection to validate that push is triggered.
	//dummyClient := adsConnectAndWait(t, 0x0a0a0a0a)
	//defer dummyClient.Close()

	return server, tearDown
}

func initLocalPilotTestEnv() (*bootstrap.Server, util.TearDownFunc) {
	return localPilotTestEnv(func(server *bootstrap.Server) {
		// Service and endpoints for hello.default - used in v1 pilot tests
		hostname := host.Name("hello.default.svc.cluster.local")
		server.EnvoyXdsServer.MemRegistry.AddService(hostname, &model.Service{
			Hostname: hostname,
			Address:  "10.10.0.3",
			Ports:    testPorts(0),
			Attributes: model.ServiceAttributes{
				Name:      "local",
				Namespace: "default",
			},
		})

		server.EnvoyXdsServer.MemRegistry.SetEndpoints(string(hostname), "default", []*model.IstioEndpoint{
			{
				Address:         "127.0.0.1",
				EndpointPort:    uint32(testEnv.Ports().BackendPort),
				ServicePortName: "http",
				Locality:        model.Locality{Label: "az"},
				ServiceAccount:  "hello-sa",
			},
		})

		// "local" service points to the current host and the in-process mixer http test endpoint
		hostname = "local.default.svc.cluster.local"
		server.EnvoyXdsServer.MemRegistry.AddService(hostname, &model.Service{
			Hostname: hostname,
			Address:  "10.10.0.4",
			Ports: []*model.Port{
				{
					Name:     "http",
					Port:     80,
					Protocol: protocol.HTTP,
				}},
			Attributes: model.ServiceAttributes{
				Name:      "local",
				Namespace: "default",
			},
		})

		server.EnvoyXdsServer.MemRegistry.SetEndpoints(string(hostname), "default", []*model.IstioEndpoint{
			{
				Address:         localIP,
				EndpointPort:    uint32(testEnv.Ports().BackendPort),
				ServicePortName: "http",
				Locality:        model.Locality{Label: "az"},
			},
		})

		// Explicit test service, in the v2 memory registry. Similar with mock.MakeService,
		// but easier to read.
		hostname = "service3.default.svc.cluster.local"
		server.EnvoyXdsServer.MemRegistry.AddService(hostname, &model.Service{
			Hostname: hostname,
			Address:  "10.10.0.1",
			Ports:    testPorts(0),
			Attributes: model.ServiceAttributes{
				Name:      "service3",
				Namespace: "default",
			},
		})

		svc3Endpoints := make([]*model.IstioEndpoint, len(testPorts(0)))
		for i, p := range testPorts(0) {
			svc3Endpoints[i] = &model.IstioEndpoint{
				Address:         app3Ip,
				EndpointPort:    uint32(p.Port),
				ServicePortName: p.Name,
				Locality:        model.Locality{Label: "az"},
			}
		}

		server.EnvoyXdsServer.MemRegistry.SetEndpoints(string(hostname), "default", svc3Endpoints)

		// Mock ingress service
		server.EnvoyXdsServer.MemRegistry.AddService("istio-ingress.istio-system.svc.cluster.local", &model.Service{
			Hostname: "istio-ingress.istio-system.svc.cluster.local",
			Address:  "10.10.0.2",
			Ports: []*model.Port{
				{
					Name:     "http",
					Port:     80,
					Protocol: protocol.HTTP,
				},
				{
					Name:     "https",
					Port:     443,
					Protocol: protocol.HTTPS,
				},
			},
			// TODO: set attribute for this service. It may affect TestLDSIsolated as we now having service defined in istio-system namespaces
		})
		server.EnvoyXdsServer.MemRegistry.AddInstance("istio-ingress.istio-system.svc.cluster.local", &model.ServiceInstance{
			Endpoint: &model.IstioEndpoint{
				Address:         ingressIP,
				EndpointPort:    80,
				ServicePortName: "http",
				Locality:        model.Locality{Label: "az"},
				Labels:          labels.Instance{constants.IstioLabel: constants.IstioIngressLabelValue},
			},
			ServicePort: &model.Port{
				Name:     "http",
				Port:     80,
				Protocol: protocol.HTTP,
			},
		})
		server.EnvoyXdsServer.MemRegistry.AddInstance("istio-ingress.istio-system.svc.cluster.local", &model.ServiceInstance{
			Endpoint: &model.IstioEndpoint{
				Address:         ingressIP,
				EndpointPort:    443,
				ServicePortName: "https",
				Locality:        model.Locality{Label: "az"},
				Labels:          labels.Instance{constants.IstioLabel: constants.IstioIngressLabelValue},
			},
			ServicePort: &model.Port{
				Name:     "https",
				Port:     443,
				Protocol: protocol.HTTPS,
			},
		})

		// RouteConf Service4 is using port 80, to test that we generate multiple clusters (regression)
		// service4 has no endpoints
		server.EnvoyXdsServer.MemRegistry.AddHTTPService("service4.default.svc.cluster.local", "10.1.0.4", 80)
	})
}

func envoyInit() {
	statsURL := fmt.Sprintf("http://localhost:%d/stats?format=json", testEnv.Ports().AdminPort)
	res, err := http.Get(statsURL)
	if err != nil {
		log.Errora("Failed to get stats, envoy not started")
	}
	statsBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errora("Failed to get stats, envoy not started")
	}

	statsMap := stats2map(statsBytes)

	if statsMap["cluster_manager.cds.update_success"] < 1 {
		log.Errora("Failed cds update")
	}
	// Other interesting values for CDS: cluster_added: 19, active_clusters
	// cds.update_attempt: 2, cds.update_rejected, cds.version
	for _, port := range testPorts(0) {
		stat := fmt.Sprintf("cluster.outbound|%d||service3.default.svc.cluster.local.update_success", port.Port)
		if statsMap[stat] < 1 {
			log.Errora("Failed cds updates")
		}
	}
	if statsMap["cluster.xds-grpc.update_failure"] > 0 {
		log.Errora("GRPC update failure")
	}
	if statsMap["listener_manager.lds.update_rejected"] > 0 {
		log.Errora("LDS update failure")
	}
	if statsMap["listener_manager.lds.update_success"] < 1 {
		log.Errora("LDS update failure")
	}
}

func testService() {
	proxyURL, _ := url.Parse("http://localhost:17002")
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	res, err := client.Get("http://local.default.svc.cluster.local")
	if err != nil {
		log.Errora("Failed to access proxy", err)
		return
	}
	resdmp, _ := httputil.DumpResponse(res, true)
	log.Infoa(string(resdmp))
	if res.Status != "200 OK" {
		log.Errora("Proxy failed ", res.Status)
	}
}

func TestEnvoy(stopCh chan T) {
	_, tearDown := initLocalPilotTestEnv()
	defer func() {
		if testEnv != nil {
			testEnv.TearDown()
		}
	}()
	startEnvoy()
	// Make sure tcp port is ready before starting the test.
	env.WaitForPort(testEnv.Ports().TCPProxyPort)

	envoyInit()
	testService()

	select {
		case <-stopCh:
		default:
			time.Sleep(5 * time.Second)
	}
}

type T int

func main(){

	stopCh := make(chan T)
	TestEnvoy(stopCh)
}