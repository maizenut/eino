package channel

import (
	"strings"
	"testing"

	declarative "github.com/cloudwego/eino/schema/declarative"
)

func TestChannelSpecValidate_RequiredFields(t *testing.T) {
	if err := (*ChannelSpec)(nil).Validate(); err == nil {
		t.Fatal("nil spec must error")
	}
	if err := (&ChannelSpec{}).Validate(); err == nil {
		t.Fatal("missing info.name must error")
	}
	spec := &ChannelSpec{Info: Info{Name: "x"}}
	if err := spec.Validate(); err == nil {
		t.Fatal("missing transport must error")
	}
}

func TestChannelSpecValidate_StdioCommandOptional(t *testing.T) {
	spec := &ChannelSpec{
		Info:     Info{Name: "console"},
		Endpoint: EndpointSpec{Transport: TransportStdio},
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("stdio without command must be valid: %v", err)
	}
	spec.Endpoint.Command = "echo"
	if err := spec.Validate(); err != nil {
		t.Fatalf("stdio with command: %v", err)
	}
}

func TestChannelSpecValidate_WebSocketRequiresAddress(t *testing.T) {
	spec := &ChannelSpec{
		Info:     Info{Name: "ws"},
		Endpoint: EndpointSpec{Transport: TransportWebSocket},
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("expected error for missing address")
	}
	spec.Endpoint.Address = "ws://localhost:8080"
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestChannelSpecValidate_CronJob(t *testing.T) {
	graphRef := &declarative.Ref{Kind: declarative.RefKindGraphDocument, Target: "g.json"}

	spec := &ChannelSpec{
		Info:     Info{Name: "cron"},
		Endpoint: EndpointSpec{Transport: TransportCronJob},
		GraphRef: graphRef,
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("missing schedule must error")
	}

	spec.Endpoint.Metadata = map[string]any{"schedule": ""}
	if err := spec.Validate(); err == nil {
		t.Fatal("empty schedule must error")
	}

	spec.Endpoint.Metadata = map[string]any{"schedule": "@every 5m"}
	if err := spec.Validate(); err != nil {
		t.Fatalf("valid cronjob: %v", err)
	}

	spec.GraphRef = nil
	if err := spec.Validate(); err == nil {
		t.Fatal("missing graph_ref must error")
	}
}

func TestChannelSpecValidate_RejectsScriptTransports(t *testing.T) {
	for _, tr := range []string{"bash", "python", "shell", "command", "script"} {
		spec := &ChannelSpec{
			Info:     Info{Name: "scripted"},
			Endpoint: EndpointSpec{Transport: tr},
		}
		err := spec.Validate()
		if err == nil {
			t.Fatalf("transport %q must be rejected", tr)
		}
		if !strings.Contains(err.Error(), "graph node runnable") {
			t.Fatalf("transport %q error = %v, want guidance toward graph node runnable", tr, err)
		}
	}
}
