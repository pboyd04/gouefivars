/*
Package vars implements a simple library for accessing UEFI variables via go.
*/
package vars

import (
	"encoding/binary"
	"fmt"

	"github.com/google/uuid"
)

// A UefiVariable defines a variable in the UEFI space
type UefiVariable struct {
	// The GUID that defines this variable's vendor according to the UEFI spec
	GUID uuid.UUID
	// The name of the variable
	Name string

	//For some reason I seem to have to get all the data from the windows variables all at once... this lets me save the data
	private interface{}
}

// Attributes gets the attributes for the variable
func (u UefiVariable) Attributes() (uint, error) {
	return getAttributeForVar(&u)
}

// Raw gets the byte array version of the variable data
func (u UefiVariable) Raw() ([]byte, error) {
	return getRawValueForVar(&u)
}

// String gets the string version of the variable data
func (u UefiVariable) String() (string, error) {
	rawData, err := u.Raw()
	if err != nil {
		return "", err
	}
	//TODO handle CHAR16...
	return string(rawData), nil
}

// Uint gets the uint version of the variable data
func (u UefiVariable) Uint() (uint64, error) {
	rawData, err := u.Raw()
	if err != nil {
		return 0, err
	}
	switch len(rawData) {
	case 1:
		return uint64(rawData[0]), nil
	case 2:
		return uint64(binary.LittleEndian.Uint16(rawData)), nil
	case 3:
		var ret uint64
		ret += uint64(rawData[0])
		ret += uint64(rawData[1]) << 8
		ret += uint64(rawData[2]) << 16
		return ret, nil
	case 4:
		return uint64(binary.BigEndian.Uint32(rawData)), nil
	case 8:
		return binary.BigEndian.Uint64(rawData), nil
	default:
		return 0, fmt.Errorf("unable to convert array of len %d to uint64", len(rawData))
	}
}

// SetRaw sets the byte array data of the variable
func (u UefiVariable) SetRaw(value []byte, attributes uint) error {
	return setRawValueForVar(&u, value, attributes)
}
