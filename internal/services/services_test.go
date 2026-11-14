package services

import (
	"strings"
	"testing"

	"github.com/rauni5/gclone/internal/project"
)

func TestParseServiceURL(t *testing.T) {
	kind, u := parseServiceURL("postgres://app:secret@localhost:5433/shop")
	if kind != "postgres" || u == nil || u.Port() != "5433" {
		t.Fatalf("postgres url parsed wrong: %s %v", kind, u)
	}
	kind, u = parseServiceURL("amqp://guest:guest@localhost:5672/vh")
	if kind != "rabbitmq" || u == nil || u.Port() != "5672" || u.Hostname() != "localhost" {
		t.Fatalf("amqp url parsed wrong: %s %v", kind, u)
	}
	kind, u = parseServiceURL("amqps://user:pass@mq.example.net:5671/vh")
	if kind != "rabbitmq" || u == nil || u.Hostname() != "mq.example.net" {
		t.Fatalf("amqps url parsed wrong: %s %v", kind, u)
	}
	if kind, _ := parseServiceURL("mongodb://h1:27017,h2:27017/db?replicaSet=rs"); kind != "mongodb" {
		t.Errorf("replica set url should still be mongodb, got %q", kind)
	}
	if kind, _ := parseServiceURL("https://example.com"); kind != "" {
		t.Errorf("http url should not match, got %q", kind)
	}
}

func TestDetectNeeds(t *testing.T) {
	p := &project.Project{Pkgs: []*project.Pkg{{
		Rel:      "server",
		DepKinds: map[string]string{"mongodb": "mongoose"},
		EnvVars: []project.EnvVar{
			{Key: "MONGODB_URI", Value: "mongodb://localhost:27018/app", Kind: "ok"},
			{Key: "DATABASE_URL", Value: "postgres://u:p@db.example.net:5432/x", Kind: "review"},
			{Key: "PLACEHOLDER_URL", Value: "mysql://user:password@host:3306/db", Kind: "blocking"},
		},
	}}}
	needs := DetectNeeds(p)
	if len(needs) != 3 {
		t.Fatalf("got %d needs, want 3", len(needs))
	}
	mongo, pg, my := needs[0], needs[1], needs[2]
	if mongo.Kind != "mongodb" || mongo.Port != 27018 || mongo.Remote {
		t.Errorf("mongo need wrong: %+v", mongo)
	}
	if pg.Kind != "postgres" || !pg.Remote || pg.RemoteHost != "db.example.net" {
		t.Errorf("postgres need wrong: %+v", pg)
	}
	if my.Kind != "mysql" || my.Remote {
		t.Errorf("placeholder url must not count as remote: %+v", my)
	}
}

func TestDetectNeedsRabbitMQ(t *testing.T) {
	depOnly := DetectNeeds(&project.Project{Pkgs: []*project.Pkg{{
		Rel:      "worker",
		DepKinds: map[string]string{"rabbitmq": "amqplib"},
	}}})
	if len(depOnly) != 1 {
		t.Fatalf("got %d needs, want 1", len(depOnly))
	}
	if depOnly[0].Kind != "rabbitmq" || depOnly[0].Remote || depOnly[0].Port != 5672 {
		t.Errorf("rabbitmq dep need wrong: %+v", depOnly[0])
	}

	local := DetectNeeds(&project.Project{Pkgs: []*project.Pkg{{
		Rel: "app",
		EnvVars: []project.EnvVar{
			{Key: "AMQP_URL", Value: "amqp://localhost:5672", Kind: "ok"},
		},
	}}})
	if len(local) != 1 || local[0].Kind != "rabbitmq" || local[0].Remote {
		t.Errorf("local amqp need wrong: %+v", local)
	}

	remote := DetectNeeds(&project.Project{Pkgs: []*project.Pkg{{
		Rel: "app",
		EnvVars: []project.EnvVar{
			{Key: "AMQP_URL", Value: "amqps://broker.example.net:5671", Kind: "ok"},
		},
	}}})
	if len(remote) != 1 || remote[0].Kind != "rabbitmq" || !remote[0].Remote || remote[0].RemoteHost != "broker.example.net" {
		t.Errorf("remote amqps need wrong: %+v", remote)
	}

	nonLocal := DetectNeeds(&project.Project{Pkgs: []*project.Pkg{{
		Rel: "app",
		EnvVars: []project.EnvVar{
			{Key: "AMQP_URL", Value: "amqp://mq.internal:5672", Kind: "ok"},
		},
	}}})
	if len(nonLocal) != 1 || nonLocal[0].Kind != "rabbitmq" || !nonLocal[0].Remote || nonLocal[0].RemoteHost != "mq.internal" {
		t.Errorf("non-local amqp need wrong: %+v", nonLocal)
	}
}

func TestDockerRunArgs(t *testing.T) {
	n := &project.Need{Kind: "postgres", Port: 5432, User: "app", Pass: "secret", DB: "shop"}
	got := strings.Join(dockerRunArgs(n, "gclone-postgres"), " ")
	for _, want := range []string{"POSTGRES_USER=app", "POSTGRES_PASSWORD=secret", "POSTGRES_DB=shop", "5432:5432", "postgres:16"} {
		if !strings.Contains(got, want) {
			t.Errorf("args %q missing %q", got, want)
		}
	}

	rmq := &project.Need{Kind: "rabbitmq", Port: 5672}
	got = strings.Join(dockerRunArgs(rmq, "gclone-rabbitmq"), " ")
	for _, want := range []string{"5672:5672", "gclone-rabbitmq:/var/lib/rabbitmq", "rabbitmq:3"} {
		if !strings.Contains(got, want) {
			t.Errorf("rabbitmq args %q missing %q", got, want)
		}
	}
}

func TestImageKind(t *testing.T) {
	cases := map[string]string{
		"postgres:16": "postgres", "bitnami/postgresql:16": "postgres", "mariadb:11": "mysql",
		"mongo:7": "mongodb", "mongo-express": "", "redis:7": "redis", "rabbitmq:3": "rabbitmq", "node:22": "",
	}
	for img, want := range cases {
		if got := imageKind(img); got != want {
			t.Errorf("imageKind(%s) = %q, want %q", img, got, want)
		}
	}
}

func TestComposeProvidesRabbitMQ(t *testing.T) {
	p := &project.Project{Composes: []*project.Compose{{
		Rel:        "docker-compose.yml",
		DBServices: []string{"mq"},
		Images:     map[string]string{"mq": "rabbitmq:3-management"},
		Started:    true,
	}}}
	kinds := composeKinds(p, true)
	if !kinds["rabbitmq"] {
		t.Fatalf("expected rabbitmq to be provided by compose, got %v", kinds)
	}
}
