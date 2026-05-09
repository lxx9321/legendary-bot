package comm

import (
	"errors"
	"wechatdll/Algorithm"
)

func ValidateCarOnlineProfile(wxid string, D *LoginData) error {
	if D == nil || D.Wxid == "" || wxid == "" || D.Wxid != wxid {
		return errors.New("Wxid")
	}

	carProfileKey := "device_profile:car:" + wxid
	exists, err := RedisClient.Exists(carProfileKey).Result()
	if err != nil {
		return errors.New("car_profile")
	}
	if exists == 0 {
		return nil
	}

	carProfile, err := GetLoginata(carProfileKey, nil)
	if err != nil {
		return errors.New("car_profile")
	}
	if carProfile == nil || carProfile.Wxid == "" {
		return errors.New("car_profile")
	}
	if carProfile.DeviceType != Algorithm.CarDeviceType || carProfile.ClientVersion != int32(Algorithm.CarVersion) || carProfile.Deviceid_str == "" {
		return errors.New("car_profile")
	}
	if carProfile.DeviceInfo == nil || carProfile.DeviceInfo.DeviceID == "" {
		return errors.New("DeviceInfo.deviceid")
	}

	if D.DeviceType != carProfile.DeviceType {
		return errors.New("DeviceType")
	}
	if D.ClientVersion != carProfile.ClientVersion {
		return errors.New("ClientVersion")
	}
	if D.Deviceid_str != carProfile.Deviceid_str {
		return errors.New("Deviceid_str")
	}
	if D.Imei != carProfile.Imei {
		return errors.New("Imei")
	}
	if D.DeviceInfo == nil || D.DeviceInfo.DeviceID == "" {
		return errors.New("DeviceInfo.deviceid")
	}
	if D.DeviceInfo.DeviceID != carProfile.DeviceInfo.DeviceID {
		return errors.New("DeviceInfo.deviceid")
	}

	return nil
}
