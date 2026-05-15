//go:build ignore

package main

import (
	"fmt"
	"github.com/vivek/time-tracker/internal/config"
)

func main() {
	c, _ := config.Load("")
	fmt.Printf("IdleThresholdMinutes: %v\n", c.IdleThresholdMinutes)
	fmt.Printf("IdleThreshold(): %v\n", c.IdleThreshold())
}
