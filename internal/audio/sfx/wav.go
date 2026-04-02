package sfx

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

func WriteWAV(path string, sampleRate int, samples []int16) (err error) {
	if sampleRate <= 0 {
		return fmt.Errorf("invalid sample rate %d", sampleRate)
	}
	if sampleRate > MaxSampleRate {
		return fmt.Errorf("sample rate %d exceeds max %d", sampleRate, MaxSampleRate)
	}

	dataSize := int64(len(samples)) * 2
	riffSize := int64(36) + dataSize
	byteRate := int64(sampleRate) * 2
	if dataSize > math.MaxUint32 {
		return fmt.Errorf("wav data too large: %d bytes", dataSize)
	}
	if riffSize > math.MaxUint32 {
		return fmt.Errorf("wav riff chunk too large: %d bytes", riffSize)
	}
	if byteRate > math.MaxUint32 {
		return fmt.Errorf("wav byte rate too large: %d", byteRate)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(f)
	defer func() {
		if flushErr := bw.Flush(); err == nil && flushErr != nil {
			err = flushErr
		}
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	if _, err := bw.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint32(riffSize)); err != nil {
		return err
	}
	if _, err := bw.Write([]byte("WAVE")); err != nil {
		return err
	}
	if _, err := bw.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint32(byteRate)); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint16(2)); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint16(16)); err != nil {
		return err
	}
	if _, err := bw.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, uint32(dataSize)); err != nil {
		return err
	}
	if err := binary.Write(bw, binary.LittleEndian, samples); err != nil {
		return err
	}
	return nil
}
