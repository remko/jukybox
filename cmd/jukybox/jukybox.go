package main

import (
	"github.com/remko/jukybox"
)

func main() {
	// f, err := os.OpenFile("jukybox.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	// if err != nil {
	// 	panic(err)
	// }
	// defer f.Close()
	// log.SetOutput(f)

	jukybox.CreateApp().Run()
}
