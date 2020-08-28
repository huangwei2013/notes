package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"istio.io/istio/pilot/pkg/model"
	v3 "istio.io/istio/pilot/pkg/xds/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"istio.io/istio/pilot/pkg/bootstrap"
	"istio.io/istio/pkg/config/protocol"
	"istio.io/istio/pkg/config/mesh"
	"istio.io/istio/pkg/keepalive"
	"istio.io/istio/pkg/test/env"
	"istio.io/istio/pkg/util/gogoprotomarshal"
)

type AdsClient discovery.AggregatedDiscoveryService_StreamAggregatedResourcesClient
type TearDownFunc func()

const (
	udsPath = "/var/run/test/socket"
)

var (
	initMutex      sync.Mutex
	pilotGrpcAddr string
)

var nodeMetadata = &structpb.Struct{Fields: map[string]*structpb.Value{
	"ISTIO_VERSION": {Kind: &structpb.Value_StringValue{StringValue: "1.3"}}, // actual value doesn't matter
}}

func TestCds(){
	server := localPilotTestEnv(func(server *bootstrap.Server) {
		addUdsEndpoint(server) // add some to this server
	})

	// 使用实例
	_, port, err := net.SplitHostPort(server.GRPCListener.Addr().String())
	if err != nil {
		return
	}
	pilotGrpcAddr = "localhost:" + port

	fakeNode(pilotGrpcAddr, "app3", "10.2.0.1")
	fakeNode(pilotGrpcAddr, "app4", "10.2.0.2")
}

// make One fake node to making request
func fakeNode(pilotGrpcAddr string, deployment string, appIp string){
	cdsr, cancel, err := connectADS(pilotGrpcAddr)
	if err != nil {
		log.Fatal(err)
	}

	err = sendCDSReq(sidecarID(appIp, deployment), cdsr)
	if err != nil {
		log.Fatal(err)
	}
	defer cancel()
	res, err := cdsr.Recv()
	if err != nil {
		log.Fatal("Failed to receive CDS", err)
		return
	}

	if len(res.Resources) == 0 {
		log.Fatal("No response")
	}
	if res.Resources[0].GetTypeUrl() != v3.ClusterType {
		log.Fatalf("Unexpected type url. want: %v, got: %v", v3.ClusterType, res.Resources[0].GetTypeUrl())
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

func sidecarID(ip, deployment string) string {
	return fmt.Sprintf("sidecar~%s~%s-644fc65469-96dza.testns~testns.svc.cluster.local", ip, deployment)
}

func sendCDSReq(node string, client AdsClient) error {
	return sendXds(node, client, v3.ClusterType, "")
}
func sendXds(node string, client AdsClient, typeURL string, errMsg string) error {
	var errorDetail *status.Status
	if errMsg != "" {
		errorDetail = &status.Status{Message: errMsg}
	}
	err := client.Send(&discovery.DiscoveryRequest{
		ResponseNonce: time.Now().String(),
		Node: &corev3.Node{
			Id:       node,
			Metadata: nodeMetadata,
		},
		ErrorDetail: errorDetail,
		TypeUrl:     typeURL})
	if err != nil {
		return fmt.Errorf("%v Request failed: %s", typeURL, err)
	}

	return nil
}


func connectADS(url string) (AdsClient, TearDownFunc, error) {
	conn, err := grpc.Dial(url, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, nil, fmt.Errorf("GRPC dial failed: %s", err)
	}
	xds := discovery.NewAggregatedDiscoveryServiceClient(conn)
	client, err := xds.StreamAggregatedResources(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("stream resources failed: %s", err)
	}

	return client, func() {
		_ = client.CloseSend()
		_ = conn.Close()
	}, nil
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


func main(){
	TestCds()
}
