package main

// main.go — entry point. Bygger som ink.App.

import (
	ink "github.com/dennwc/inkview"
)

func main() {
	ink.Run(NewGame())
}
