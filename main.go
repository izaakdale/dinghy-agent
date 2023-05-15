package main

import "github.com/izaakdale/dinghy-agent/app"

func main() {
	// start a seft cluster with a static ip
	app.New().Run()
}
