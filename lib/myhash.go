package lib

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type MyHash struct {
	logger *logrus.Logger
}

func NewMyHash() *MyHash {
	return &MyHash{
		logger: LoadLogger(),
	}
}

// md532 计算32位小写md5
func (mh *MyHash) Md532(text string) string {
	if text == "" {
		mh.logger.Errorf("Md532 text is empty.")
		return ""
	}
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

// DecryptTs 解密 TS 数据
func (mh *MyHash) DecryptTs(encryptedData, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		mh.logger.Errorf("DecryptTs NewCipher error: %s", err.Error())
		return nil, err
	}
	if len(encryptedData)%aes.BlockSize != 0 {
		mh.logger.Errorln("encrypted data is not a multiple of the block size")
		return nil, errors.New("encrypted data is not a multiple of the block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encryptedData))
	mode.CryptBlocks(decrypted, encryptedData)

	// Unpad
	return mh.pkcs7Unpad(decrypted, aes.BlockSize)
}

// DetectImageFormat 检测图片格式，例如 jpeg、png
func (mh *MyHash) DetectImageFormat(imageData []byte) (string, error) {
	reader := bytes.NewReader(imageData)
	_, format, err := image.Decode(reader)
	if err != nil {
		mh.logger.Errorf("DetectImageFormat Decode error: %v", err)
		return "", err
	}
	return strings.ToLower(format), nil
}

// ImageFromDataURI 解析 data:image/... 格式的图片数据
func (mh *MyHash) ImageFromDataURI(dataURI string) ([]byte, error) {
	if !strings.HasPrefix(dataURI, "data:image/") {
		mh.logger.Errorln("invalid data URI format")
		return nil, errors.New("invalid data URI format")
	}
	split := strings.SplitN(dataURI, ",", 2)
	if len(split) != 2 {
		mh.logger.Errorln("invalid data URI format")
		return nil, errors.New("invalid data URI format")
	}
	return base64.StdEncoding.DecodeString(split[1])
}

// DecryptAESImg 解密 base64 编码的 AES CBC 数据
func (mh *MyHash) AesDecryptCBCBase6(encryptedBase64 string, key, iv []byte) (string, error) {
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		mh.logger.Errorf("DecryptAESImg DecodeString error: %v", err)
		return "", err
	}
	unpadded, err := mh.AesDecryptCBC(encryptedData, key, iv)
	if err != nil {
		mh.logger.Errorf("DecryptAESImg error: %v", err)
		return "", err
	}
	return string(unpadded), nil
}

func (mh *MyHash) AesDecryptCBC(cipherText []byte, key, iv []byte) ([]byte, error) {
	if len(cipherText)%aes.BlockSize != 0 {
		mh.logger.Errorln("cipherText data is not a multiple of the block size")
		return nil, errors.New("cipherText is not a multiple of the block size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		mh.logger.Errorf("aes NewCipher error: %v", err)
		return nil, err
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(cipherText))
	mode.CryptBlocks(plain, cipherText)
	return mh.pkcs7Unpad(plain, aes.BlockSize)
}

// pkcs7Unpad 解除 PKCS7 填充
func (mh *MyHash) pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 {
		mh.logger.Errorln("pkcs7Unpad  data is empty")
		return nil, errors.New("invalid padding size")
	}
	paddingLen := int(data[length-1])
	if paddingLen > blockSize || paddingLen == 0 {
		mh.logger.Errorln("invalid padding")
		return nil, errors.New("invalid padding")
	}
	return data[:(length - paddingLen)], nil
}

// GenerateRandomNumbers 生成 min-max 范围内的随机浮点数
func (mh *MyHash) GenerateRandomNumbers(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

// randomDuration 生成 [minMs, maxMs] 范围内的随机时间
func (mh *MyHash) RandomDuration(minMs, maxMs int) time.Duration {
	diff := maxMs - minMs
	return time.Duration(minMs + rand.Intn(diff))
}

// GenURLMD5 生成URL的MD5哈希值
func (mh *MyHash) GenURLMD5(url string) string {
	h := md5.New()
	h.Write([]byte(url))
	return hex.EncodeToString(h.Sum(nil))
}

// GetCompositeKey 生成 MD5 + 长度的复合键
func (mh *MyHash) GetCompositeKey(url string) string {
	md5Hash := mh.GenURLMD5(url)
	urlLen := len(url)
	return md5Hash + "_" + strconv.Itoa(urlLen) // MD5 字符串 + 长度字符串
}
