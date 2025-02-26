package utils

import (
	"lingzhi-server/utils/asr"
	"lingzhi-server/utils/vad"
)

func Init() error {
	err := vad.Init()
	if err != nil {
		return err
	}

	err = asr.Init()
	if err != nil {
		return err
	}

	return nil
}
