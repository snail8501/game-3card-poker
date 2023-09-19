package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mr-tron/base58"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Transaction struct {
	Type      string    `json:"type"`
	Id        string    `json:"id"`
	Execution Execution `json:"execution"`
	Fee       Fee       `json:"fee"`
}

type Execution struct {
	Transitions     []Transition `json:"transitions"`
	GlobalStateRoot string       `json:"global_state_root"`
}

type Transition struct {
	Id       string    `json:"id"`
	Program  string    `json:"program"`
	Function string    `json:"function"`
	Inputs   []Inputs  `json:"inputs"`
	Outputs  []Outputs `json:"outputs"`
	Proof    string    `json:"proof"`
	Tpk      string    `json:"tpk"`
	Tcm      string    `json:"tcm"`
}

type Inputs struct {
	Type  string `json:"type"`
	Id    string `json:"id"`
	Value string `json:"value"`
}

type Outputs struct {
	Type  string `json:"type"`
	Id    string `json:"id"`
	Value string `json:"value"`
}

type Fee struct {
	Transition      Transition `json:"transition"`
	GlobalStateRoot string     `json:"global_state_root"`
	Inclusion       string     `json:"inclusion"`
}

const (
	MissTransaction      = "Something went wrong: Missing transaction for ID"
	MissTransactionError = "miss transaction"
	TransactionBaseUrl   = "https://vm.aleo.org/api/testnet3/transaction/"
	AppName              = "game_3_card_poker"
	SavePokerName        = "save_poker"
	SaveRoundName        = "save_round"
	SaveUserBalanceName  = "save_user_balance"
)

// ExecuteMethod 执行链上对应方法，返回新Record保存到数据库
func ExecuteMethod(privateKey string, viewKey string, appName string, params []string, transition string, record string) (string, error) {
	output, err := SnarkOsExecute(privateKey, appName, params, transition, record)
	fmt.Println("执行输出: ", output)
	if err != nil {
		log.Println("SnarkOsExecute error:", err)
		return "", err
	}
	transactionId, err := ParseExecuteOutput(output)
	if len(transactionId) == 0 {
		log.Println("ParseExecuteOutput error: ", output)
		return "", nil
	}
	fmt.Println("transactionID: ", transactionId)
	if err != nil {
		log.Println("ParseExecuteOutput error:", err)
		return "", err
	}
	ciphertext, err := GetCiphertext(transactionId)
	for {
		if err == nil || err.Error() != MissTransactionError {
			break
		}
		time.Sleep(time.Millisecond * 500)
		ciphertext, err = GetCiphertext(transactionId)
	}
	fmt.Println("ciphertext: ", ciphertext)
	if err != nil {
		log.Println("GetCiphertext error:", err)
		return "", err
	}
	newRecord, err := DecryptRecord(viewKey, ciphertext)
	fmt.Println("record: ", newRecord)
	if err != nil {
		log.Println("DecryptRecord error:", err)
		return "", err
	}
	return newRecord, nil
}

// SnarkOsExecute 执行合约方法
func SnarkOsExecute(privateKey string, appName string, params []string, transition string, record string) (string, error) {
	param := ""
	for _, s := range params {
		param = fmt.Sprintf("%s %s", param, fmt.Sprintf(`"%s"`, s))
	}
	cmd := fmt.Sprintf(`snarkos developer execute "%s.aleo" "%s" %s --private-key "%s" --query "https://vm.aleo.org/api" --broadcast "https://vm.aleo.org/api/testnet3/transaction/broadcast" --fee 1000 --record "%s"`, appName, transition, param, privateKey, record)
	fmt.Println(cmd)
	result, err := Command(cmd)
	return result, err
}

// ParseExecuteOutput 解析执行合约后输出
func ParseExecuteOutput(output string) (string, error) {
	split := strings.Split(output, "\n")
	for _, s := range split {
		if strings.HasPrefix(s, "at1") {
			return strings.TrimSpace(s), nil
		}
	}
	return "", nil
}

// GetCiphertext 根据交易ID获取Ciphertext
func GetCiphertext(transactionId string) (string, error) {
	url := TransactionBaseUrl + transactionId
	fmt.Println("url:", url)
	client := http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Failed to create HTTP request:", err)
		return "", err
	}

	// 发送HTTP请求并获取响应
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Failed to send HTTP request:", err)
		return "", err
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Failed to read HTTP response:", err)
		return "", err
	}
	if strings.HasPrefix(string(body), MissTransaction) {
		return "", errors.New(MissTransactionError)
	}
	var transaction Transaction
	err = json.Unmarshal(body, &transaction)
	if err != nil {
		log.Println("Unmarshal error", err)
		return "", err
	}
	return transaction.Fee.Transition.Outputs[0].Value, nil
}

// DecryptRecord 根据Ciphertext和viewKey解析Record
func DecryptRecord(viewKey string, ciphertext string) (string, error) {
	cmd := fmt.Sprintf("snarkos developer decrypt --ciphertext %s --view-key %s", ciphertext, viewKey)
	fmt.Println("命令：", cmd)
	result, err := Command(cmd)
	return result, err
}

// Command 执行命令
func Command(arg ...string) (string, error) {
	name := "/bin/bash"
	c := "-c"
	// 根据系统设定不同的命令name
	if runtime.GOOS == "windows" {
		name = "cmd"
		c = "/C"
	}
	arg = append([]string{c}, arg...)
	cmd := exec.Command(name, arg...)

	//创建获取命令输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("Error:can not obtain stdout pipe for command:", err)
		return "", err
	}

	//执行命令
	if err := cmd.Start(); err != nil {
		log.Println("Error:The command is err,", err)
		return "", err
	}

	//读取所有输出
	bytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Println("ReadAll Stdout:", err.Error())
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		log.Println("wait:", err.Error())
		return "", err
	}

	result := string(bytes)
	return result, nil
}

// Base58Encode Base58加密并解密为整数
func Base58Encode(input string) string {
	decodedBytes, _ := base58.Decode(input)
	decodedInt := 0
	for _, b := range decodedBytes {
		decodedInt = decodedInt*256 + int(b)
	}
	return strconv.Itoa(decodedInt)
}
