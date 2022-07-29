//go:build linux

package vars

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
)

const (
	FS_IMMUTABLE_FL = 0x00000010
)

// GetAllVars gets a list of all the variables
func GetAllVars() ([]UefiVariable, error) {
	matches, err := filepath.Glob("/sys/firmware/efi/efivars/*")
	if err != nil {
		return nil, err
	}
	ret := make([]UefiVariable, len(matches))
	for i, m := range matches {
		fileName := filepath.Base(m)
		parts := strings.SplitN(fileName, "-", 2)
		ret[i].Name = parts[0]
		ret[i].GUID = uuid.MustParse(parts[1])
	}
	return ret, nil
}

func getFileForVariable(uefiVar *UefiVariable, mode int) (*os.File, error) {
	fileName := "/sys/firmware/efi/efivars/" + uefiVar.Name + "-" + uefiVar.GUID.String()
	if mode == os.O_RDWR {
		//Need to check if the file is immutable first...
		tmpFile, err := os.OpenFile(fileName, os.O_RDONLY, 0755)
		if err != nil {
			//Can't even open read only, there is something wrong...
			return nil, err
		}
		attr, err := unix.IoctlGetInt(int(tmpFile.Fd()), unix.FS_IOC_GETFLAGS)
		if err != nil {
			log.Printf("cannot get attributes for file %s: %v. Will continue...\n", fileName, err)
		} else {
			if attr&FS_IMMUTABLE_FL != 0 {
				//File is immutable, unset that...
				attr = attr & ^FS_IMMUTABLE_FL
				err = unix.IoctlSetPointerInt(int(tmpFile.Fd()), unix.FS_IOC_SETFLAGS, attr)
				if err != nil {
					log.Printf("cannot set attributes for file %s: %v. Will continue...\n", fileName, err)
				}
			}
		}
		tmpFile.Close()
	}
	return os.OpenFile(fileName, mode, 0755)
}

func getAttributeForVar(uefiVar *UefiVariable) (uint, error) {
	file, err := getFileForVariable(uefiVar, os.O_RDONLY)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	attrBytes := make([]byte, 4)
	_, err = file.Read(attrBytes)
	if err != nil {
		return 0, err
	}
	//TODO handle BE platforms...
	return uint(binary.LittleEndian.Uint32(attrBytes)), nil
}

func getRawValueForVar(uefiVar *UefiVariable) ([]byte, error) {
	file, err := getFileForVariable(uefiVar, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return data[4:], nil
}

func setRawValueForVar(uefiVar *UefiVariable, value []byte, attributes uint) error {
	file, err := getFileForVariable(uefiVar, os.O_RDWR)
	if err != nil {
		return err
	}
	defer file.Close()
	rawData := make([]byte, 4)
	binary.LittleEndian.PutUint32(rawData, uint32(attributes))
	rawData = append(rawData, value...)
	_, err = file.Write(rawData)
	return err
}

// GetVarByNameAndGUID get the variable with the specified name and GUID
func GetVarByNameAndGUID(name string, guid uuid.UUID) (*UefiVariable, error) {
	ret := new(UefiVariable)
	ret.Name = name
	ret.GUID = guid
	file, err := getFileForVariable(ret, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ret, nil
}

// GetVarByName gets the variable with the name specified
func GetVarByName(name string, strict bool) (*UefiVariable, error) {
	matches, err := filepath.Glob("/sys/firmware/efi/efivars/" + name + "-*")
	if err != nil {
		return nil, err
	}
	if strict && len(matches) > 1 {
		return nil, fmt.Errorf("more than one match in strict mode")
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("could not locate variable")
	}
	match := filepath.Clean(matches[0])
	if !strings.HasPrefix(match, "/sys/firmware/efi/efivars/") {
		return nil, fmt.Errorf("bad name string specified")
	}
	file, err := os.OpenFile(match, os.O_RDONLY, 0755)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	v := new(UefiVariable)
	base := filepath.Base(match)
	parts := strings.SplitN(base, "-", 2)
	v.Name = parts[0]
	v.GUID = uuid.MustParse(parts[1])
	return v, nil
}
