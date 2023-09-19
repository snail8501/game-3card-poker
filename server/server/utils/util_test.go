package utils

import (
	"fmt"
	"game-3-card-poker/server/service"
	"log"
	"testing"
	"time"
)

const privateKey = "APrivateKey1zkp7dKkghREXKtrsEHfVedfSXWskF6FHdKzw3yJPNdFZpyV"
const viewKey = "AViewKey1gpGdtNdyaZp1oPE8rR16RRNZuJtfU6HUYarfCMqGDfQb"
const address = "aleo1l44dfmwcu7j2e26yhrlhrxla4lsrmr0rxymxfzxj6h8m2mnegyqs8x0end"
const appName = "hello_9ttzuvzqr59dny"

var record = "{\n  owner: aleo1l44dfmwcu7j2e26yhrlhrxla4lsrmr0rxymxfzxj6h8m2mnegyqs8x0end.private,\n  microcredits: 17031515u64.private,\n  _nonce: 8052187601127286545381485952462234167421324300595587637729215050049661619147group.public\n}"

func TestSaveUserPoker(t *testing.T) {
	poker1 := service.UserPoker{
		OnePoker: service.Poker{
			Value: 1,
			Color: 1,
		},
		TowPoker: service.Poker{
			Value: 2,
			Color: 2,
		},
		ThreePoker: service.Poker{
			Value: 3,
			Color: 3,
		},
	}

	poker2 := service.UserPoker{
		OnePoker: service.Poker{
			Value: 11,
			Color: 1,
		},
		TowPoker: service.Poker{
			Value: 12,
			Color: 2,
		},
		ThreePoker: service.Poker{
			Value: 13,
			Color: 4,
		},
	}

	m := make(map[int64]service.UserPoker)
	m[1001] = poker1
	m[1001] = poker2
	time.Sleep(time.Minute * 5)
}

func TestBase58Encode(t *testing.T) {
	encode := Base58Encode("qwer")
	fmt.Println(encode)
}

func TestSnarkOsExecute(t *testing.T) {
	res, err := SnarkOsExecute(privateKey, appName, []string{"1u32", "2u32", "3u32"}, "main", record)
	if err != nil {
		log.Fatalln("SnarkOsExecute error", err)
	}
	fmt.Println(res)
}

func TestParseExecuteOutput(t *testing.T) {
	s, err := ParseExecuteOutput("\n📦 Creating execution transaction for 'hello_9ttzuvzqr59dny.aleo'...\n\n✅ Created execution transaction for 'hello_9ttzuvzqr59dny.aleo/main'\n✅ Successfully broadcast execution at1mqxsppv79whmdzwlthsxve4drnw6d92hkt3277p670dgd8wymgxsa3547u ('hello_9ttzuvzqr59dny.aleo/main') to https://vm.aleo.org/api/testnet3/transaction/broadcast.\nat1mqxsppv79whmdzwlthsxve4drnw6d92hkt3277p670dgd8wymgxsa3547u\n")
	if err != nil {
		log.Fatalln("ParseOutput error", err)
	}
	fmt.Printf("交易ID：%s\n", s)
}

func TestGetCiphertext(t *testing.T) {
	ciphertext, err := GetCiphertext("at1lmlcnpz8s92uvwfk4grv9u9r02pnqgclmwv4u076ctgktdjznq8qsn477l")
	if err != nil {
		log.Fatalln("GetCiphertext error", err)
	}
	fmt.Println(ciphertext)
}

func TestDecryptRecord(t *testing.T) {
	decryptRecord, err := DecryptRecord(viewKey, "record1qyqsqaez77rj0l2an4hmcmmv4kkv83y9ptna7v592qd3ecjna2sfvpcpqyxx66trwfhkxun9v35hguerqqpqzqrdaqz5aedzx7fa4r4uayncwaqshkxsu50823z5j2jh7t6j95wtqqkjxqksa930a5ag9xlefq26y8rt2fhzha984hlya2g3qk6yrndscaf6el7")
	if err != nil {
		log.Fatalln("DecryptRecord error", err)
	}
	fmt.Println(decryptRecord)
	record = decryptRecord
}
