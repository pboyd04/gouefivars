package main

import (
	"flag"
	"fmt"

	"github.com/google/uuid"
	"github.com/pboyd04/gouefivars/lib/uefi/vars"
)

func main() {
	list := flag.Bool("l", false, "List variables")
	help := flag.Bool("h", false, "Print this help message")
	str := flag.Bool("string", false, "Print the variable as a string")
	intPrint := flag.Bool("int", false, "Print the variable as an int")
	name := flag.String("name", "", "The name of the variable")
	guidStr := flag.String("guid", "", "The guid of the variable")
	flag.Parse()

	if *help {
		flag.CommandLine.Usage()
	} else if *list {
		varList, err := vars.GetAllVars()
		if err != nil {
			fmt.Printf("Failed to get all variables: %v\n", err)
		}
		for _, v := range varList {
			fmt.Printf("Name: %s Guid: %s\n", v.Name, v.GUID)
		}
		return
	} else if len(*name) > 0 && len(*guidStr) > 0 {
		//Get the variable by name and guid...
		guid, err := uuid.Parse(*guidStr)
		if err != nil {
			fmt.Printf("Failed to parse GUID: %v\n", err)
			return
		}
		v, err := vars.GetVarByNameAndGUID(*name, guid)
		if err != nil {
			fmt.Printf("Failed to get variable: %v\n", err)
			return
		}
		printVar(v, *str, *intPrint)
	} else if len(*name) > 0 {
		//Get the variable by name
		v, err := vars.GetVarByName(*name, false)
		if err != nil {
			fmt.Printf("Failed to get variable: %v\n", err)
			return
		}
		printVar(v, *str, *intPrint)
	} else {
		flag.CommandLine.Usage()
	}
}

func printVar(v *vars.UefiVariable, str bool, intPrint bool) {
	attr, err := v.Attributes()
	if err != nil {
		fmt.Printf("Failed to get attributes: %v\n", err)
	}
	fmt.Printf("Name: %s Guid: %s Attributes: %x\n", v.Name, v.GUID, attr)
	if str {
		strData, err := v.String()
		if err != nil {
			fmt.Printf("Failed to get data: %v\n", err)
		}
		fmt.Printf("Value: %s\n", strData)
	} else if intPrint {
		intData, err := v.Uint()
		if err != nil {
			fmt.Printf("Failed to get data")
		}
		fmt.Printf("Value: %x\n", intData)
	} else {
		rawData, err := v.Raw()
		if err != nil {
			fmt.Printf("Failed to get data")
		}
		fmt.Printf("Value: %v\n", rawData)
	}
}
