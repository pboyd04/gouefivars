//go:build windows

package vars

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/google/uuid"
)

//We need to get the appropriate privilege...
func getPriv() error {
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	RtlAdjustPrivilege := ntdll.NewProc("RtlAdjustPrivilege")
	var enabled uint32
	ret, _, _ := RtlAdjustPrivilege.Call(22, 1, 0, uintptr(unsafe.Pointer(&enabled)))
	if ret != 0 {
		return fmt.Errorf("requires administrator privilege")
	}
	return nil
}

func getULong(array []byte, offset uint) uint {
	var ret uint
	ret += uint(array[offset])
	ret += uint(array[offset+1]) << 8
	ret += uint(array[offset+2]) << 16
	ret += uint(array[offset+3]) << 24
	return ret
}

type variablePrivateData struct {
	Value      []byte
	Attributes uint
}

func getGUIDFromByteSlice(data []byte) (uuid.UUID, error) {
	//The google uuid library assumes BE, these guid are LE
	newGUIDBytes := make([]byte, 16)
	copy(newGUIDBytes, data)
	//First 32-bits
	newGUIDBytes[0] = data[3]
	newGUIDBytes[1] = data[2]
	newGUIDBytes[2] = data[1]
	newGUIDBytes[3] = data[0]
	//Second 16-bits
	newGUIDBytes[4] = data[5]
	newGUIDBytes[5] = data[4]
	//Third 16-bits
	newGUIDBytes[6] = data[7]
	newGUIDBytes[7] = data[6]
	//Rest is fine...
	return uuid.FromBytes(newGUIDBytes)
}

//Data is layed out like this
//DWORD NextOffset
//DWORD ValueOffset
//DWORD ValueLength
//DWORD Attributes
//byte[16] Guid
//char[] Name
//byte[] Value
func getUefiVariableFromByteStream(stream []byte, offset uint) (UefiVariable, uint, error) {
	var ret UefiVariable
	nextOffset := getULong(stream, offset)
	valueOffset := getULong(stream, offset+4)
	valueLength := getULong(stream, offset+8)
	value := stream[offset+valueOffset : offset+valueOffset+valueLength]
	attributes := getULong(stream, offset+12)
	guid, err := getGUIDFromByteSlice(stream[offset+16 : offset+32])
	if err != nil {
		return ret, 0, err
	}
	ret.GUID = guid
	var nameBuff []byte
	nameBuff = stream[offset+32 : offset+valueOffset]
	//Last char is null terminator, we need to not include that
	utf16Buff := make([]uint16, (len(nameBuff)/2)-1)
	for i := 0; i < len(utf16Buff); i++ {
		utf16Buff[i] = uint16(nameBuff[i*2]) + (uint16(nameBuff[(i*2)+1]) << 8)
	}
	name := utf16.Decode(utf16Buff)
	ret.Name = string(name)
	//Some of these seem to be double null terminated...
	if ret.Name[len(ret.Name)-1] == 0 {
		ret.Name = ret.Name[:len(ret.Name)-1]
	}
	private := new(variablePrivateData)
	private.Attributes = attributes
	private.Value = value
	ret.private = private
	if nextOffset == 0 {
		return ret, 0, nil
	}
	return ret, nextOffset + offset, nil
}

// GetAllVars gets a list of all the variables
func GetAllVars() ([]UefiVariable, error) {
	err := getPriv()
	if err != nil {
		return nil, err
	}
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	NtEnumerateSystemEnvironmentValuesEx := ntdll.NewProc("NtEnumerateSystemEnvironmentValuesEx")
	var size uint32
	_, _, err = NtEnumerateSystemEnvironmentValuesEx.Call(2, uintptr(unsafe.Pointer(nil)), uintptr(unsafe.Pointer(&size)))
	buffer := make([]byte, size)
	var pBuffer *byte
	pBuffer = &buffer[0]
	_, _, err = NtEnumerateSystemEnvironmentValuesEx.Call(2, uintptr(unsafe.Pointer(pBuffer)), uintptr(unsafe.Pointer(&size)))
	retVal := make([]UefiVariable, 0)
	uVar, nextOffset, err := getUefiVariableFromByteStream(buffer, 0)
	if err != nil {
		return nil, err
	}
	retVal = append(retVal, uVar)
	for nextOffset != 0 {
		uVar, nextOffset, err = getUefiVariableFromByteStream(buffer, nextOffset)
		if err != nil {
			return nil, err
		}
		retVal = append(retVal, uVar)
	}

	return retVal, nil
}

func getAttributeForVar(uefiVar *UefiVariable) (uint, error) {
	data := uefiVar.private.(*variablePrivateData)
	if data == nil {
		return 0, fmt.Errorf("no variable data")
	}
	return data.Attributes, nil
}

func getRawValueForVar(uefiVar *UefiVariable) ([]byte, error) {
	data := uefiVar.private.(*variablePrivateData)
	if data == nil {
		return nil, fmt.Errorf("no variable data")
	}
	return data.Value, nil
}

type winUnicodeString struct {
	Length    uint16
	MaxLength uint16
	Buffer    *uint16
}

func getWindowsUnicodeString(str string) (winUnicodeString, error) {
	var winStr winUnicodeString
	strPtr, err := syscall.UTF16FromString(str)
	if err != nil {
		return winStr, err
	}
	winStr.Length = uint16(len(strPtr) * 2)
	winStr.MaxLength = winStr.Length
	winStr.Buffer = &strPtr[0]
	return winStr, nil
}

func getWindowsGUID(g uuid.UUID) (syscall.GUID, error) {
	var winGUID syscall.GUID
	gBytes, _ := g.MarshalBinary()
	winGUID.Data1 = binary.BigEndian.Uint32(gBytes[0:4])
	winGUID.Data2 = binary.BigEndian.Uint16(gBytes[4:6])
	winGUID.Data3 = binary.BigEndian.Uint16(gBytes[6:8])
	winGUID.Data4[0] = gBytes[8]
	winGUID.Data4[1] = gBytes[9]
	winGUID.Data4[2] = gBytes[10]
	winGUID.Data4[3] = gBytes[11]
	winGUID.Data4[4] = gBytes[12]
	winGUID.Data4[5] = gBytes[13]
	winGUID.Data4[6] = gBytes[14]
	winGUID.Data4[7] = gBytes[15]
	return winGUID, nil
}

func setRawValueForVar(uefiVar *UefiVariable, value []byte, attributes uint) error {
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	NtSetSystemEnvironmentValueEx := ntdll.NewProc("NtSetSystemEnvironmentValueEx")
	pName, err := getWindowsUnicodeString(uefiVar.Name)
	if err != nil {
		return err
	}
	pGUID, err := getWindowsGUID(uefiVar.GUID)
	if err != nil {
		return err
	}
	rc, _, _ := NtSetSystemEnvironmentValueEx.Call(uintptr(unsafe.Pointer(&pName)), uintptr(unsafe.Pointer(&pGUID)), uintptr(unsafe.Pointer(&value[0])), uintptr(len(value)), uintptr(attributes))
	if rc != 0 {
		return fmt.Errorf("failed with return code %x", rc)
	}
	return nil
}

// GetVarByNameAndGUID get the variable with the specified name and GUID
func GetVarByNameAndGUID(name string, guid uuid.UUID) (*UefiVariable, error) {
	//While there do exist commands to get single vars in windows, I can't get them to work...
	varList, err := GetAllVars()
	if err != nil {
		return nil, err
	}
	for _, v := range varList {
		g1, _ := guid.MarshalBinary()
		g2, _ := v.GUID.MarshalBinary()
		if strings.Compare(name, v.Name) == 0 && bytes.Equal(g1, g2) {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("could not locate variable")
}

// GetVarByName gets the variable with the name specified
func GetVarByName(name string, strict bool) (*UefiVariable, error) {
	//While there do exist commands to get single vars in windows, I can't get them to work...
	varList, err := GetAllVars()
	if err != nil {
		return nil, err
	}
	found := false
	var holder *UefiVariable
	for _, v := range varList {
		if strings.Compare(name, v.Name) == 0 {
			if !strict {
				return &v, nil
			} else if found {
				return nil, fmt.Errorf("found more than one variable with name " + name)
			} else {
				found = true
				holder = &v
			}
		}
	}
	if holder != nil {
		return holder, nil
	}
	return nil, fmt.Errorf("could not locate variable")
}
