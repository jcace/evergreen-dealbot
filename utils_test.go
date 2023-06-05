package main

import (
	"testing"
)

func TestGenerateCarFileName(t *testing.T) {
	carDestination := "/root/cars/"
	pieceCid := "baga6ea4seaqaab6tksql2jqotldtr5zjjocnesygt725sals2ggge6ldmwmu2nq"
	expectedFileName := "/root/cars/baga6ea4seaqaab6tksql2jqotldtr5zjjocnesygt725sals2ggge6ldmwmu2nq.car"

	generatedFileName := GenerateCarFileName(carDestination, pieceCid)

	if generatedFileName != expectedFileName {
		t.Errorf("Generates Car filename correctly: got %q, expected %q", generatedFileName, expectedFileName)
	}
}
