package main

import (
	"os"
	"testing"
)

func TestParseConcurrencyFlagEmpty(t *testing.T) {
	c, err := parseConcurrency("")
	if err != nil {
		t.Fatalf("parseConcurrency(\"\") debería funcionar sin error: %s", err)
	}
	if len(c) > 0 {
		t.Fatalf("esperaba 0 configuraciones, obtuve %d", len(c))
	}
}

func TestParseConcurrencyFlagSimple(t *testing.T) {
	c, err := parseConcurrency("foo=2")
	if err != nil {
		t.Fatalf("parseConcurrency(\"foo=2\") debería funcionar sin error: %s", err)
	}
	if len(c) != 1 {
		t.Fatalf("esperaba 1 configuración, obtuve %d", len(c))
	}
	if c["foo"] != 2 {
		t.Fatalf("esperaba foo=2, obtuve foo=%d", c["foo"])
	}
}

func TestParseConcurrencyFlagMultiple(t *testing.T) {
	c, err := parseConcurrency("foo=2,bar=3")
	if err != nil {
		t.Fatalf("parseConcurrency(\"foo=2,bar=3\") debería funcionar sin error: %s", err)
	}
	if len(c) != 2 {
		t.Fatalf("esperaba 2 configuraciones, obtuve %d", len(c))
	}
	if c["foo"] != 2 {
		t.Fatalf("esperaba foo=2, obtuve foo=%d", c["foo"])
	}
	if c["bar"] != 3 {
		t.Fatalf("esperaba bar=3, obtuve bar=%d", c["bar"])
	}
}

func TestParseConcurrencyFlagWhitespace(t *testing.T) {
	c, err := parseConcurrency("foo   =   2, bar = 3")
	if err != nil {
		t.Fatalf("parseConcurrency con espacios no debería fallar: %s", err)
	}
	if len(c) != 2 {
		t.Fatalf("esperaba 2 configuraciones, obtuve %d", len(c))
	}
	if c["foo"] != 2 {
		t.Fatalf("esperaba foo=2, obtuve foo=%d", c["foo"])
	}
	if c["bar"] != 3 {
		t.Fatalf("esperaba bar=3, obtuve bar=%d", c["bar"])
	}
}

func TestParseConcurrencyFlagInvalid(t *testing.T) {
	cases := []string{
		"foo=x",
		"foo===2",
		"foobar",
		"foo=",
		"=",
		"=1",
		",",
		",,,",
	}
	for _, input := range cases {
		if _, err := parseConcurrency(input); err == nil {
			t.Fatalf("esperaba error para %q, pero no lo hubo", input)
		}
	}
}

func TestParseConcurrencyZeroValue(t *testing.T) {
	c, err := parseConcurrency("foo=0,bar=0")
	if err != nil {
		t.Fatalf("parseConcurrency(\"foo=0,bar=0\") no debería fallar: %s", err)
	}
	if c["foo"] != 0 || c["bar"] != 0 {
		t.Fatalf("esperaba foo=0 y bar=0, obtuve foo=%d, bar=%d", c["foo"], c["bar"])
	}
}

func TestParseConcurrencyOnlySpaces(t *testing.T) {
	c, err := parseConcurrency("   ")
	if err != nil {
		t.Fatalf("parseConcurrency con solo espacios no debería fallar: %s", err)
	}
	if len(c) != 0 {
		t.Fatalf("esperaba 0 configuraciones para solo espacios, obtuve %d", len(c))
	}
}

func TestPortFromEnv(t *testing.T) {
	// Caso por defecto
	os.Unsetenv("PORT")
	env := make(Env)
	port, err := basePort(env)
	if err != nil {
		t.Fatalf("no pudo obtener puerto base: %s", err)
	}
	if port != defaultPort {
		t.Fatalf("esperaba puerto %d, obtuve %d", defaultPort, port)
	}

	// Con variable de entorno
	os.Setenv("PORT", "4000")
	port, err = basePort(env)
	if err != nil {
		t.Fatalf("no pudo leer PORT=4000: %s", err)
	}
	if port != 4000 {
		t.Fatalf("esperaba puerto 4000, obtuve %d", port)
	}

	// Con env map
	env["PORT"] = "6000"
	port, err = basePort(env)
	if err != nil {
		t.Fatalf("no pudo leer env[\"PORT\"]=6000: %s", err)
	}
	if port != 6000 {
		t.Fatalf("esperaba puerto 6000, obtuve %d", port)
	}

	// Valor no entero
	env["PORT"] = "mango"
	if _, err := basePort(env); err == nil {
		t.Fatal("esperaba error al leer PORT=\"mango\", pero no lo hubo")
	}
}

func TestBasePortFlagPriority(t *testing.T) {
	// Respaldar y restaurar flagPort
	old := flagPort
	defer func() { flagPort = old }()
	flagPort = 7000

	env := make(Env)
	port, err := basePort(env)
	if err != nil {
		t.Fatalf("basePort con flagPort no debería fallar: %s", err)
	}
	if port != 7000 {
		t.Fatalf("esperaba puerto 7000, obtuve %d", port)
	}
}

func TestReadConfigFileOverride(t *testing.T) {
	var procfile = "Profile"
	var port = 5000
	var concurrency string = "web=2"
	var gracetime int = 3
	// Se asume que ./fixtures/configs/.mango existe con valores de prueba
	err := readConfigFile("./fixtures/configs/.mango", &procfile, &port, &concurrency, &gracetime, &flagLokiURL, &flagLokiJob)
	if err != nil {
		t.Fatalf("no pudo leer ./fixtures/configs/.mango: %s", err)
	}
	if procfile != "Procfile.dev" {
		t.Fatalf("esperaba Procfile.dev, obtuve %q", procfile)
	}
	if port != 15000 {
		t.Fatalf("esperaba puerto 15000, obtuve %d", port)
	}
	if concurrency != "foo=2,bar=3,web=3" {
		t.Fatalf("esperaba concurrency=\"foo=2,bar=3,web=3\", obtuve %q", concurrency)
	}
	if gracetime != 30 {
		t.Fatalf("esperaba gracetime=30, obtuve %d", gracetime)
	}
}

func TestReadConfigFileNotExist(t *testing.T) {
	pf, p, c, g := "Pf", 1234, "x=1", 5
	if err := readConfigFile("./no_such_file", &pf, &p, &c, &g, &flagLokiURL, &flagLokiJob); err == nil {
		t.Fatal("esperaba error al leer config inexistente, pero no lo hubo")
	}
}
