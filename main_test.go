package main

import (
	"testing"
)

func TestEnvIntFallbackEmpty(t *testing.T) {
	t.Setenv("ENVTEST_INT", "")
	if envInt("ENVTEST_INT", 7) != 7 {
		t.Fatal()
	}
}

func TestEnvIntParses(t *testing.T) {
	t.Setenv("ENVTEST_INT2", " 42 ")
	if envInt("ENVTEST_INT2", 0) != 42 {
		t.Fatal()
	}
}

func TestEnvIntInvalidUsesFallback(t *testing.T) {
	t.Setenv("ENVTEST_INT3", "nope")
	if envInt("ENVTEST_INT3", 99) != 99 {
		t.Fatal()
	}
}

func TestEnvStringFallback(t *testing.T) {
	t.Setenv("ENVTEST_S", "")
	if envString("ENVTEST_S", "d") != "d" {
		t.Fatal()
	}
}

func TestEnvStringTrim(t *testing.T) {
	t.Setenv("ENVTEST_S2", "  x  ")
	if envString("ENVTEST_S2", "") != "x" {
		t.Fatal()
	}
}

func TestEnvCSVNilWhenUnset(t *testing.T) {
	t.Setenv("ENVTEST_CSV", "")
	if envCSV("ENVTEST_CSV") != nil {
		t.Fatal()
	}
}

func TestEnvCSVSplits(t *testing.T) {
	t.Setenv("ENVTEST_CSV2", " a , b ")
	v := envCSV("ENVTEST_CSV2")
	if len(v) != 2 || v[0] != "a" || v[1] != "b" {
		t.Fatal(v)
	}
}

func TestEnvCSVSkipsEmptyParts(t *testing.T) {
	t.Setenv("ENVTEST_CSV3", ",z,")
	v := envCSV("ENVTEST_CSV3")
	if len(v) != 1 || v[0] != "z" {
		t.Fatal(v)
	}
}
