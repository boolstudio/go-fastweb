// vim: set syntax=go autoindent:
// Copyright 2010 Ivan Wong. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
        "go-fastweb.googlecode.com/svn/trunk/src/fastweb"
        "os"
)

type Products struct {
	fastweb.Controller
	Name string
	Brand string
	Features []string
	Specifications []string
	Image string
}

func (p *Products) View(id string) os.Error {
	if id == "ah64" {
		p.Name = "RC Apache AH64 4-Channel Electric Helicoper"
		p.Brand = "Colco"
		p.Features = []string{
			"4 channel radio control duel propeller system",
			"Full movement controll: forward, backward, left, right, up and down",
			"Replica design",
			"Revolutionary co-axial rotor technology",
		}
		p.Specifications = []string{
			"Dimensions: L 16 Inches X W 5.5 Inches x H 6.5 Inches",
			"Battery Duration: 10 min",
			"Range: 120 Feet",
		}
		p.Image = "/img/ah64.jpg"
	}
	return nil
}

func main() {
	a := fastweb.NewApplication()
	a.RegisterController(&Products{})
        a.Run(":12345")
}

