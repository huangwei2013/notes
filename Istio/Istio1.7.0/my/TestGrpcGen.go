package main

import (
	"context"
	"istio.io/pkg/log"
	"os"
	"time"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	ads "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"

	networking "istio.io/api/networking/v1alpha3"

	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/grpcgen"
	"istio.io/istio/pilot/pkg/xds"
	v2 "istio.io/istio/pilot/pkg/xds/v2"
	"istio.io/istio/pkg/config/schema/collections"

	_ "google.golang.org/grpc/xds"
)


var (
	grpcAddr = "127.0.0.1:14057"

	// Address of the Istiod gRPC service, used in tests.
	istiodSvcAddr = "istiod.istio-system.svc.cluster.local:14057"

)


func runGRPC(){
	isStop := false

	ds := xds.NewXDS()
	ds.DiscoveryServer.Generators["grpc"] = &grpcgen.GrpcConfigGenerator{}
	epGen := &xds.EdsGenerator{Server: ds.DiscoveryServer}
	ds.DiscoveryServer.Generators["grpc/"+v2.EndpointType] = epGen

	sd := ds.DiscoveryServer.MemRegistry
	sd.AddHTTPService("fortio1.fortio.svc.cluster.local", "127.0.0.1", 8081)
	sd.AddHTTPService("istiod.istio-system.svc.cluster.local", "127.0.0.1", 14057)
	sd.SetEndpoints("istiod.istio-system.svc.cluster.local", "", []*model.IstioEndpoint{
		{
			Address:         "127.0.0.1",
			EndpointPort:    uint32(14057),
			ServicePortName: "http-main",
		},
	})
	se := collections.IstioNetworkingV1Alpha3Serviceentries.Resource()

	store := ds.MemoryConfigStore
	store.Create(model.Config{
		ConfigMeta: model.ConfigMeta{
			GroupVersionKind: se.GroupVersionKind(),
			Name:             "fortio",
			Namespace:        "fortio",
		},
		Spec: &networking.ServiceEntry{
			Hosts: []string{
				"fortio.fortio.svc",
				"fortio.fortio.svc.cluster.local",
			},
			Addresses: []string{"1.2.3.4"},

			Ports: []*networking.Port{
				{Number: 14057, Name: "grpc-insecure", Protocol: "http"},
			},

			Endpoints: []*networking.WorkloadEntry{
				{
					Address: "127.0.0.1",
					Ports:   map[string]uint32{"grpc-insecure": 8080},
				},
			},
			Location:   networking.ServiceEntry_MESH_EXTERNAL,
			Resolution: networking.ServiceEntry_STATIC,
		},
	})

	env := ds.DiscoveryServer.Env
	if err := env.PushContext.InitContext(env, env.PushContext, nil); err != nil {
		log.Errora(err)
		return
	}
	ds.DiscoveryServer.UpdateServiceShards(env.PushContext)

	err := ds.StartGRPC(grpcAddr)
	if err != nil {
		log.Errora(err)
		return
	}
	defer ds.GRPCListener.Close()

	os.Setenv("GRPC_XDS_BOOTSTRAP", "/go/src/istio.io/istio/pilot/pkg/networking/grpcgen/testdata/xds_bootstrap.json")

	// gRPC-resolve
	go func() {
		rb := resolver.Get("xds") // xds-experimental
		if rb == nil {
			log.Errora("[gRPC-resolve] Failed to Get resolver ")
			isStop = true
			return
		}

		ch := make(chan resolver.State)
		_, err := rb.Build(resolver.Target{Endpoint: istiodSvcAddr},
			&testClientConn{ch: ch}, resolver.BuildOptions{})
		if err != nil {
			log.Errora("[gRPC-resolve] Failed to resolve XDS ", err)
			return
		}

		tm := time.After(10 * time.Second)
		select {
		case s := <-ch:
			log.Infoa("[gRPC-resolve] Got state ", s)
		// TODO: timeout
		case <-tm:
			log.Errora("[gRPC-resolve] Didn't resolve")
			isStop = true
			return
		}
	}()

	// gRPC-cdslb
	go func() {
		rb := balancer.Get("eds_experimental")
		if rb == nil {
			log.Errora("[gRPC-cdslb] Failed to Get resolver ")
			isStop = true
			return
		}

		b := rb.Build(&testLBClientConn{}, balancer.BuildOptions{})
		defer b.Close()
	}()

	// gRPC-dial
	go func() {
		conn, err := grpc.Dial("xds:///istiod.istio-system.svc.cluster.local:14057", grpc.WithInsecure())
		if err != nil {
			log.Errora("[gRPC-dial] XDS gRPC Dial", err)
			isStop = true
			return
		}

		defer conn.Close()
		xds := ads.NewAggregatedDiscoveryServiceClient(conn)

		s, err := xds.StreamAggregatedResources(context.Background())
		if err != nil {
			log.Errora("[gRPC-dial] XDS gRPC StreamAggregatedResources", err)
			isStop = true
			return
		}
		log.Infoa("[gRPC-dial] DiscoveryRequest Send : ",s.Send(&xdsapi.DiscoveryRequest{}))
	}()


	for {
		if isStop { break }
		time.Sleep(5 * time.Second)
	}
}

type testLBClientConn struct {
	balancer.ClientConn
}

// From xds_resolver_test
// testClientConn is a fake implemetation of resolver.ClientConn. All is does
// is to store the state received from the resolver locally and signal that
// event through a channel.
type testClientConn struct {
	resolver.ClientConn
	ch chan resolver.State
}

func (t *testClientConn) UpdateState(s resolver.State) {
	log.Infoa("testClientConn.UpdateState")
	t.ch <- s
}

func (t *testClientConn) ReportError(err error) {
	log.Infoa("testClientConn.ReportError")
}

func (t *testClientConn) ParseServiceConfig(jsonSC string) *serviceconfig.ParseResult {
	// Will be called with something like:
	//
	//	"loadBalancingConfig":[
	//	{
	//		"cds_experimental":{
	//			"Cluster": "istiod.istio-system.svc.cluster.local:14056"
	//		}
	//	}
	//]
	//}
	log.Infoa("testClientConn.ParseServiceConfig")
	return &serviceconfig.ParseResult{}
}


func main(){

	runGRPC()


}
