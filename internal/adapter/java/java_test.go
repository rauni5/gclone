package java

import (
	"reflect"
	"testing"
)

func TestParseJavaMajor(t *testing.T) {
	cases := map[string]int{
		`openjdk version "17.0.9" 2023-10-17`: 17,
		`java version "1.8.0_392"`:            8,
		`openjdk version "21" 2023-09-19`:     21,
		`command not found`:                   0,
	}
	for in, want := range cases {
		if got := ParseJavaMajor(in); got != want {
			t.Errorf("ParseJavaMajor(%q)=%d want %d", in, got, want)
		}
	}
}

func TestParseBuildMaven(t *testing.T) {
	b := ParseBuild("maven", `<parent><artifactId>spring-boot-starter-parent</artifactId></parent>
<properties><java.version>1.8</java.version></properties>
<dependency><artifactId>mysql-connector-j</artifactId></dependency>
<modules><module>api</module></modules>`)
	if b.JavaWants != 8 || !b.SpringBoot || !b.MultiModule || b.DepKinds["mysql"] == "" {
		t.Errorf("unexpected: %+v", b)
	}
}

func TestParseBuildGradle(t *testing.T) {
	cases := map[string]int{
		"java { toolchain { languageVersion = JavaLanguageVersion.of(21) } }": 21,
		"sourceCompatibility = '17'":                                          17,
		"sourceCompatibility = JavaVersion.VERSION_11":                        11,
		"kotlin { jvmToolchain(17) }":                                         17,
	}
	for in, want := range cases {
		if got := ParseBuild("gradle", in).JavaWants; got != want {
			t.Errorf("%q: got %d want %d", in, got, want)
		}
	}
	if !ParseBuild("gradle", `id("org.springframework.boot") version "3.3.0"`).SpringBoot {
		t.Error("Spring Boot plugin not detected")
	}
}

func TestFlattenYAMLAndProperties(t *testing.T) {
	y := FlattenYAML("server:\n  port: 8081 # dev\nspring:\n  datasource:\n    url: \"jdbc:mysql://localhost/x\"\n  application:\n    name: demo\n---\nother: ignored\n")
	want := [][2]string{{"server.port", "8081"}, {"spring.datasource.url", "jdbc:mysql://localhost/x"}, {"spring.application.name", "demo"}}
	if !reflect.DeepEqual(y, want) {
		t.Errorf("yaml: got %v", y)
	}
	p := ParseProperties("# c\nserver.port=9090\nspring.datasource.username : demo\n")
	if len(p) != 2 || p[0] != [2]string{"server.port", "9090"} || p[1][1] != "demo" {
		t.Errorf("properties: got %v", p)
	}
}

func TestResolve(t *testing.T) {
	cases := []struct{ in, val, env string }{
		{"${DB_PASSWORD}", "", "DB_PASSWORD"},
		{"${PORT:8080}", "8080", ""},
		{"${URL:jdbc:postgresql://localhost/db}", "jdbc:postgresql://localhost/db", ""},
		{"plain", "plain", ""},
	}
	for _, c := range cases {
		v, e := resolve(c.in)
		if v != c.val || e != c.env {
			t.Errorf("resolve(%q)=(%q,%q) want (%q,%q)", c.in, v, e, c.val, c.env)
		}
	}
}
