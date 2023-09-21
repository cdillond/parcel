package main

import (
	"encoding/gob"
)

func EncodeGob(o string, res Result) error {
	f, err := OutFile(o)
	if err != nil {
		return err
	}
	enc := gob.NewEncoder(f)
	err = enc.Encode(res)
	if err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
