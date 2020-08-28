package main

import (
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"istio.io/istio/pilot/pkg/bootstrap"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/adsc"
	"istio.io/istio/pkg/config/mesh"
	"istio.io/istio/pkg/config/protocol"
	"istio.io/istio/pkg/keepalive"
	"istio.io/istio/pkg/test/env"
	"istio.io/istio/pkg/util/gogoprotomarshal"
)

const (
	asdcLocality  = "region1/zone1/subzone1"
	asdc2Locality = "region2/zone2/subzone2"

	edsIncSvc = "eds.test.svc.cluster.local"
	edsIncVip = "10.10.1.2"

	udsPath = "/var/run/test/socket"
)

var (
	initMutex      sync.Mutex
	localIP 	= "10.3.0.3"
	adscIP		= "10.10.10.10"

	pilotGrpcAddr string
)


func TestEds(stopCh chan T){
	// 启动实例
	server := localPilotTestEnv(func(server *bootstrap.Server) {
		addUdsEndpoint(server) // add some to this server
	})

	// 使用实例
	_, port, err := net.SplitHostPort(server.GRPCListener.Addr().String())
	if err != nil {
		return
	}
	pilotGrpcAddr = "localhost:" + port

	adscConn := adsConnectAndWait(adscIP)
	if adscConn == nil {
		log.Fatalln(" adscConn return nil, exiting...")
		return
	}
	defer adscConn.Close()

	// 更成形的场景测试
	//testTCPEndpoints("127.0.0.1", adscConn)
	//testUdsEndpoints(server, adscConn)

	for {
		select {
		case <-stopCh:
			return
		default:
			time.Sleep(5 * time.Second)
		}
	}
}

func getHttpAddr() string{
	pilotHTTP := os.Getenv("PILOT_HTTP")
	if len(pilotHTTP) == 0 {
		pilotHTTP = "0"
	}
	httpAddr := ":" + pilotHTTP
	return httpAddr
}

func getMeshFile()(f *os.File, err error){
	// Create tmp mesh config file
	meshFile, err := ioutil.TempFile("", "mesh.yaml")
	if err != nil {
		log.Fatalln("creating tmp mesh config file failed: %v", err)
		return nil, err
	}
	defer meshFile.Close()
	meshConfig := mesh.DefaultMeshConfig()
	meshConfig.DefaultConfig.DiscoveryAddress = "localhost:15012"
	meshConfig.EnableAutoMtls.Value = false
	meshConfig.EnableTracing = true
	data, err := gogoprotomarshal.ToYAML(&meshConfig)
	if err != nil {
		os.Remove(meshFile.Name())
		log.Fatalln("meshConfig marshal failed: %v", err)
		return nil, err
	}
	meshFile.Write([]byte(data))

	return meshFile, nil
}

func getAdditionalArgs(additionalArgs ...func(*bootstrap.PilotArgs)) []func(*bootstrap.PilotArgs) {

	meshFile, _ := getMeshFile()

	additionalArgs = append(additionalArgs, func(args *bootstrap.PilotArgs) {
		args.Plugins = bootstrap.DefaultPlugins
	})
	additionalArgs = append([]func(p *bootstrap.PilotArgs){func(p *bootstrap.PilotArgs) {
		p.Namespace = "istio-system"
		p.ServerOptions = bootstrap.DiscoveryServerOptions{
			HTTPAddr:        getHttpAddr(),
			GRPCAddr:        ":0",
			EnableProfiling: true,
		}
		p.RegistryOptions = bootstrap.RegistryOptions{
			KubeConfig: env.IstioSrc + "/tests/util/kubeconfig",
			// Static testdata, should include all configs we want to test.
			FileDir: env.IstioSrc + "/tests/testdata/config",
		}
		p.MCPOptions.MaxMessageSize = 1024 * 1024 * 4
		p.KeepaliveOptions = keepalive.DefaultOption()
		p.MeshConfigFile = meshFile.Name()

		// TODO: add the plugins, so local tests are closer to reality and test full generation
		// Plugins:           bootstrap.DefaultPlugins,
	}}, additionalArgs...)

	return additionalArgs
}

func localPilotTestEnv(initFunc func(*bootstrap.Server), additionalArgs ...func(*bootstrap.PilotArgs)) (*bootstrap.Server) { //nolint: unparam
	log.Println("localPilotTestEnv Start")
	initMutex.Lock()
	defer initMutex.Unlock()

	additionalArgs = getAdditionalArgs(additionalArgs...)
	args := bootstrap.NewPilotArgs(additionalArgs...)

	stop := make(chan struct{})

	// 创建实例 & 运行
	server, _ := bootstrap.NewServer(args)
	if err := server.Start(stop); err != nil {
		return nil
	}

	// Run the initialization function.
	initFunc(server)

	// Trigger a push, to initiate push context with contents of registry.
	server.EnvoyXdsServer.Push(&model.PushRequest{Full: true})

	// Wait till a push is propagated.
	time.Sleep(200 * time.Millisecond)

	log.Println("localPilotTestEnv Done")
	return server
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}


// Verify server sends the endpoint. This check for a single endpoint with the given
// address.
func testTCPEndpoints(expected string, adsc *adsc.ADSC) {
	testEndpoints(expected, "outbound|8080||eds.test.svc.cluster.local", adsc)
}
// Verify server sends the endpoint. This check for a single endpoint with the given
// address.
func testEndpoints(expected string, cluster string, adsc *adsc.ADSC) {
	lbe, f := adsc.GetEndpoints()[cluster]
	if !f || len(lbe.Endpoints) == 0 {
		log.Fatalf("No lb endpoints for %v, %v", cluster, adsc.EndpointsJSON())
	}
	var found []string
	for _, lbe := range lbe.Endpoints {
		for _, e := range lbe.LbEndpoints {
			addr := e.GetEndpoint().Address.GetSocketAddress().Address
			found = append(found, addr)
			if expected == addr {
				return
			}
		}
	}
	log.Fatal("Expecting %s got %v", expected, found)
	if len(found) != 1 {
		log.Fatal("Expecting 1, got ", len(found))
	}
}

// Verify server sends UDS endpoints
func testUdsEndpoints(_ *bootstrap.Server, adsc *adsc.ADSC) {
	// Check the UDS endpoint ( used to be separate test - but using old unused GRPC method)
	// The new test also verifies CDS is pusing the UDS cluster, since adsc.eds is
	// populated using CDS response
	log.Println("testUdsEndpoints : ")
	lbe, f := adsc.GetEndpoints()["outbound|0||localuds.cluster.local"]
	if !f || len(lbe.Endpoints) == 0 {
		log.Fatalf("No UDS lb endpoints")
	} else {
		ep0 := lbe.Endpoints[0]
		if len(ep0.LbEndpoints) != 1 {
			log.Fatalf("expected 1 LB endpoint but got %d", len(ep0.LbEndpoints))
		}
		lbep := ep0.LbEndpoints[0]
		path := lbep.GetEndpoint().GetAddress().GetPipe().GetPath()
		if path != udsPath {
			log.Fatalf("expected Pipe to %s, got %s", udsPath, path)
		}
	}
}

func addUdsEndpoint(server *bootstrap.Server) {
	server.EnvoyXdsServer.MemRegistry.AddService("localuds.cluster.local", &model.Service{
		Hostname: "localuds.cluster.local",
		Ports: model.PortList{
			{
				Name:     "grpc",
				Port:     0,
				Protocol: protocol.GRPC,
			},
		},
		MeshExternal: true,
		Resolution:   model.ClientSideLB,
	})
	server.EnvoyXdsServer.MemRegistry.AddInstance("localuds.cluster.local", &model.ServiceInstance{
		Endpoint: &model.IstioEndpoint{
			Address:         udsPath,
			EndpointPort:    0,
			ServicePortName: "grpc",
			Locality:        model.Locality{Label: "localhost"},
			Labels:          map[string]string{"socket": "unix"},
		},
		ServicePort: &model.Port{
			Name:     "grpc",
			Port:     0,
			Protocol: protocol.GRPC,
		},
	})
}

func adsConnectAndWait(ip string) *adsc.ADSC {
	log.Println("adsConnectAndWait : ", pilotGrpcAddr)
	adscConn, err := adsc.Dial(pilotGrpcAddr ,"", &adsc.Config{
		IP: ip,
	})
	if err != nil {
		log.Fatal("Error connecting ", err)
		return nil
	}
	adscConn.Watch()
	_, err = adscConn.Wait(10*time.Second, "eds", "lds", "cds", "rds")
	if err != nil {
		log.Fatal("Error getting initial config ", err)
		return nil
	}

	if len(adscConn.GetEndpoints()) == 0 {
		log.Fatal("No endpoints")
		return nil
	}
	return adscConn
}

type T int
func main(){

	stopCh := make(chan T)
	TestEds(stopCh)
}
