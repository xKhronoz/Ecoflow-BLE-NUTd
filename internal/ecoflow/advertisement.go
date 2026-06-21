package ecoflow

import (
	"fmt"
)

const ecoFlowManufacturerID = 0xB5B5

func ParseScanRecord(data []ManufacturerData) (ScanRecord, error) {
	for _, md := range data {
		if md.CompanyID != ecoFlowManufacturerID {
			continue
		}
		return parseScanRecordBytes(md.Data)
	}
	return ScanRecord{}, fmt.Errorf("missing EcoFlow manufacturer data")
}

func parseScanRecordBytes(data []byte) (ScanRecord, error) {
	if len(data) < 17 {
		return ScanRecord{}, fmt.Errorf("manufacturer data too short")
	}
	record := ScanRecord{
		ProtoVersion: data[0],
		SerialNumber: string(data[1:17]),
	}
	if len(data) > 17 {
		record.Status = data[17]
	}
	if len(data) > 18 {
		record.ProductType = data[18]
	}
	if len(data) > 22 {
		record.CapabilityFlags = data[22]
	} else {
		record.CapabilityFlags = 0b0111000
	}
	record.Encrypt = record.CapabilityFlags&0b0000001 != 0
	record.SupportVerified = record.CapabilityFlags&0b0000010 != 0
	record.Verified = record.CapabilityFlags&0b0000100 != 0
	record.EncryptType = int((record.CapabilityFlags & 0b0111000) >> 3)
	record.Support5G = record.CapabilityFlags&0b1000000 != 0
	record.ActiveFlag = ((record.Status >> 7) & 0x01) == 1
	return record, nil
}
