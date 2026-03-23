package config

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGenerator_static(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: static
    value: hello
endpoints:
  e:
    path: /
    method: GET
`)
	g, err := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	if err != nil {
		t.Fatal(err)
	}
	v, err := g.Generate()
	if err != nil || v != "hello" {
		t.Fatalf("got %v %v", v, err)
	}
}

func TestGenerator_randomInt(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: randomInt
    min: 5
    max: 5
endpoints:
  e:
    path: /
    method: GET
`)
	g, _ := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	v, _ := g.Generate()
	if v != 5 {
		t.Fatalf("got %v", v)
	}
}

func TestGenerator_formattedInt(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: formattedInt
    min: 2
    max: 2
    format: "id_{}"
endpoints:
  e:
    path: /
    method: GET
`)
	g, _ := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	v, _ := g.Generate()
	if v != "id_2" {
		t.Fatalf("got %v", v)
	}
}

func TestGenerator_choice(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: choice
    values: [a, b]
endpoints:
  e:
    path: /
    method: GET
`)
	g, _ := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	for i := 0; i < 20; i++ {
		v, err := g.Generate()
		if err != nil {
			t.Fatal(err)
		}
		s := fmt.Sprint(v)
		if s != "a" && s != "b" {
			t.Fatalf("got %q", s)
		}
	}
}

func TestGenerator_randomString_charset(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: randomString
    length: 12
    charset: hex
endpoints:
  e:
    path: /
    method: GET
`)
	g, _ := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	v, _ := g.Generate()
	s := v.(string)
	if len(s) != 12 {
		t.Fatalf("len %d", len(s))
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("bad char %q", c)
		}
	}
}

func TestGenerator_template_paramsAlias(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: template
    template: "{{a}}-{{b}}"
    params:
      a:
        type: static
        value: "1"
      b:
        type: static
        value: "2"
endpoints:
  e:
    path: /
    method: GET
`)
	g, err := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	if err != nil {
		t.Fatal(err)
	}
	v, _ := g.Generate()
	if v != "1-2" {
		t.Fatalf("got %v", v)
	}
}

func TestGenerator_object_fieldsAlias(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: object
    fields:
      n:
        type: static
        value: 7
endpoints:
  e:
    path: /
    method: GET
`)
	g, err := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	if err != nil {
		t.Fatal(err)
	}
	v, _ := g.Generate()
	m := v.(map[string]any)
	if fmt.Sprint(m["n"]) != "7" {
		t.Fatalf("got %#v", m)
	}
}

func TestGenerator_array(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: array
    minLength: 2
    maxLength: 2
    elementGenerator:
      type: static
      value: z
endpoints:
  e:
    path: /
    method: GET
`)
	g, _ := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	v, _ := g.Generate()
	a := v.([]any)
	if len(a) != 2 || fmt.Sprint(a[0]) != "z" {
		t.Fatalf("got %#v", a)
	}
}

func TestGenerator_refUsesRegisteredInstance(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  seq:
    type: sequence
    start: 10
    increment: 1
endpoints:
  e:
    path: /
    method: GET
`)
	ref := &ReferenceGenerator{Name: "seq", Config: cfg}
	a, _ := ref.Generate()
	b, _ := ref.Generate()
	if a != 10 || b != 11 {
		t.Fatalf("got %v %v", a, b)
	}
}

func TestGenerator_sequence_format(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: sequence
    start: 1
    increment: 1
    format: "N{}"
endpoints:
  e:
    path: /
    method: GET
`)
	g, _ := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	v1, _ := g.Generate()
	v2, _ := g.Generate()
	if v1 != "N1" || v2 != "N2" {
		t.Fatalf("%v %v", v1, v2)
	}
}

func TestGenerator_sequence_concurrent(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: sequence
    start: 0
    increment: 1
endpoints:
  e:
    path: /
    method: GET
`)
	g, _ := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = g.Generate()
		}()
	}
	wg.Wait()
}

func TestGenerator_uuid(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	g := &UUIDGenerator{}
	v, err := g.Generate()
	if err != nil {
		t.Fatal(err)
	}
	s := v.(string)
	if !re.MatchString(s) {
		t.Fatalf("bad uuid %q", s)
	}
}

func TestGenerator_timestamp_unix(t *testing.T) {
	g := &TimestampGenerator{Format: "unix"}
	v, err := g.Generate()
	if err != nil {
		t.Fatal(err)
	}
	sec := v.(int64)
	if sec < time.Now().Unix()-5 || sec > time.Now().Unix()+5 {
		t.Fatalf("unix %d", sec)
	}
}

func TestGenerator_timestamp_rfc3339(t *testing.T) {
	g := &TimestampGenerator{Format: "rfc3339"}
	v, err := g.Generate()
	if err != nil {
		t.Fatal(err)
	}
	_, err = time.Parse(time.RFC3339Nano, v.(string))
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenerator_randomFloat(t *testing.T) {
	cfg := testCfg(t, `
parameterGenerators:
  x:
    type: randomFloat
    minFloat: 1.0
    maxFloat: 1.0
    precision: 2
endpoints:
  e:
    path: /
    method: GET
`)
	g, _ := cfg.createGeneratorFromDef(cfg.ParameterGenerators["x"])
	v, _ := g.Generate()
	f := v.(float64)
	if math.Abs(f-1.0) > 1e-9 {
		t.Fatalf("got %v", f)
	}
}

func TestGenerator_randomBool_distribution(t *testing.T) {
	g := &RandomBoolGenerator{PTrue: 0.0}
	for i := 0; i < 10; i++ {
		v, _ := g.Generate()
		if v.(bool) {
			t.Fatal("expected false")
		}
	}
	g2 := &RandomBoolGenerator{PTrue: 1.0}
	for i := 0; i < 10; i++ {
		v, _ := g2.Generate()
		if !v.(bool) {
			t.Fatal("expected true")
		}
	}
}

func TestCanonical_random_intAlias(t *testing.T) {
	pe := NewParameterEngine()
	gen, err := pe.createGeneratorWithConfig(map[string]any{
		"type": "random_int",
		"min":  3,
		"max":  3,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := gen.Generate()
	if v != 3 {
		t.Fatalf("got %v", v)
	}
}

func testCfg(t *testing.T, yaml string) *Config {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(p, []byte(strings.TrimSpace(`
baseUrls:
  - http://localhost
execution:
  mode: fixed
  durationSeconds: 1
  requestsPerSecond: 1
  requestTimeoutMs: 1000
`)+"\n"+yaml), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
