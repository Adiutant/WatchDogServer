package main

import (
	regexps "RegExpTime"
	checksum "checksum-master"
	crand "crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	_ "go-sqlite3"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"
)

type subject struct {
	id    string
	name  string
	state string
	bpm   int
	age   int
}
type subjectXml struct {
	XMLName xml.Name `xml:"subject"`
	Id      string   `xml:"id"`
	Name    string   `xml:"name"`
	State   string   `xml:"state"`
	Bpm     int      `xml:"bpm"`
	Age     int      `xml:"age"`
}
type newSubjectPacket struct {
	XMLName    xml.Name `xml:"newsubject"`
	Name       string   `xml:"name"`
	Age        int      `xml:"age"`
	ServerCode string   `xml:"servercode"`
}

const ONLINE_STATUS = "ONLINE"
const JUST_REGISTRED_STATUS = "JUST_REGISTRED"

var infoLog = log.New(os.Stdin, "INFO\t", log.Ldate|log.Ltime)

var subjects []subject

const subjectsStoragePath = "SUBJECTS.db"
const SERVERCODE_PATH = "server_code.scode"

var serverCode = ""

//var subjectsTable =

func generateServerKey() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		infoLog.Println("Не удалось сформировать код сервера")
		return "", err
	}
	var codeVector = hostname + time.Now().Format("2006-01-02-15_04_05") + strconv.FormatInt(NewCryptoRand(), 10)
	file, err := os.Create("tempchsum.tmp")
	if err != nil {
		infoLog.Println("Не удалось сформировать код сервера")
		return "", err
	}
	ioutil.WriteFile(file.Name(), []byte(codeVector), 0666)
	serverCode, _ := checksum.SHA256sum(file.Name())
	file.Close()
	os.Remove(file.Name())
	return serverCode, nil

}
func getServerKey() []byte {
	file, err := os.Open(SERVERCODE_PATH)
	if err != nil {
		infoLog.Fatal("НЕ НАЙДЕН ФАЙЛ КОДА СЕРВЕРА ФАТАЛЬНАЯ ОШИБКА")
	}
	serverCode, _ := ioutil.ReadFile(file.Name())
	file.Close()
	return serverCode
}
func NewCryptoRand() int64 {
	safeNum, err := crand.Int(crand.Reader, big.NewInt(100234))
	if err != nil {
		panic(err)
	}
	return safeNum.Int64()
}
func logInit() error {
	logFileName := "log" + time.Now().Format("2006-01-02-15_04_05") + ".txt"
	dir, dirErr := os.ReadDir("Logs")
	if dirErr != nil || dir == nil {
		err := os.Mkdir("Logs", 0666)
		if err != nil {
			return err
		}
	}
	logFile, logErr := os.OpenFile(logFileName, os.O_WRONLY, 0666)
	logFile, logErr = os.Create("Logs/" + logFileName)
	if logFile == nil {
		return logErr
	}
	infoLog = log.New(logFile, "INFO\t", log.Ldate|log.Ltime)
	return nil
}
func httpServerInit() error {
	go http.HandleFunc("/updateSubject", updateSubject)
	go http.HandleFunc("/newSubject", newSubjectRegistration)
	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		return err
	}
	return nil

}
func generateSubjectCode(name string, age int) [20]byte {
	vector := name + strconv.Itoa(age) + strconv.Itoa(int(NewCryptoRand()))
	return sha1.Sum([]byte(vector))

}
func newSubjectRegistration(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		var newSubject newSubjectPacket
		//var body []byte
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			infoLog.Println("ОШИБКА ПОЛУЧЕНИЯ ТЕЛА")
			infoLog.Println(err)
		}
		infoLog.Println(string(body))
		if err := xml.Unmarshal(body, &newSubject); err != nil {
			infoLog.Println("Получен некорректный XML файл")
			infoLog.Println(err)
			infoLog.Println(string(body))
			w.WriteHeader(404)

			//panic(err)
		} else {
			var unhexedServerCode []byte
			hex.Decode(unhexedServerCode, []byte(newSubject.ServerCode))
			if string(unhexedServerCode) == string(getServerKey()) {
				subjectCode := generateSubjectCode(newSubject.Name, newSubject.Age)
				w.Write(subjectCode[:])
				db, err := sql.Open("sqlite3", subjectsStoragePath)
				if err != nil {
					infoLog.Println("Проблемы с базой данных")
				}
				currentTime := time.Now()

				status := JUST_REGISTRED_STATUS + "P" + currentTime.Format(regexps.TIME_TEMPLATE)

				queryPrep := fmt.Sprintf("insert into SUBJECTS (id,name,state,bpm,age) values (\"%s\",\"%s\",\"%s\",%v,%v)", string(subjectCode[:]), newSubject.Name, status, 0, 0)
				db.Exec(queryPrep)
				if err != nil {
					infoLog.Println("Проблемы с базой данных")
					infoLog.Println(err)
					//infoLog.Println(query)
					db.Close()
				}
				db.Close()
			} else {
				infoLog.Println("ПОЛУЧЕН НЕВЕРНЫЙ КОД СЕРВЕРА")
			} //here /////////////////////////////////////////////////////////////
		}
	}
}
func updateSubject(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {

		var receivedSubject subjectXml
		//var body []byte
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			infoLog.Println("ОШИБКА ПОЛУЧЕНИЯ ТЕЛА")
			infoLog.Println(err)
		}
		infoLog.Println(string(body))
		if err := xml.Unmarshal(body, &receivedSubject); err != nil {
			infoLog.Println("Получен некорректный XML файл")
			infoLog.Println(err)
			infoLog.Println(string(body))
			w.WriteHeader(404)

			//panic(err)
		} else {
			infoLog.Println("Обновление данных")
			err := subjectStatusChange(receivedSubject.Id, receivedSubject.State)
			if err != nil {
				infoLog.Println("ERROR ")
				w.WriteHeader(404)
			}
			err = statusInfoUpdate(receivedSubject.Id, receivedSubject.Age, receivedSubject.Bpm)
			if err != nil {
				infoLog.Println("ERROR ")
				w.WriteHeader(404)
			}
		}

	}
}
func statusInfoUpdate(subjId string, age int, bpm int) error {
	db, err := sql.Open("sqlite3", subjectsStoragePath)
	if err != nil {
		return err
	}
	// queryPrep := fmt.Sprintf("select * from SUBJECTS where id=\"%s\"", subjId)
	// row := db.QueryRow(queryPrep)
	// if err != nil {
	// 	infoLog.Println("Проблемы с базой данных")
	// 	infoLog.Println(err)
	// 	db.Close()
	// 	return err
	// }
	queryPrep := fmt.Sprintf("update SUBJECTS SET age=%v,bpm=%v where id=\"%s\"", age, bpm, subjId)
	db.Exec(queryPrep)
	if err != nil {
		infoLog.Println("Проблемы с базой данных")
		infoLog.Println(err)
		//infoLog.Println(query)
		db.Close()
		return err
	}
	db.Close()
	return nil

}

func subjectStatusChange(subjId string, status string) error {
	db, err := sql.Open("sqlite3", subjectsStoragePath)
	if err != nil {
		infoLog.Println(err)
		return err
	}
	queryPrep := fmt.Sprintf("select * from SUBJECTS where id=\"%s\"", subjId)

	row := db.QueryRow(queryPrep)
	if err != nil {
		infoLog.Println("Проблемы с базой данных")
		infoLog.Println(err)
		db.Close()
		return err
	}

	proxySubject := subject{}
	err = row.Scan(&proxySubject.id, &proxySubject.name, &proxySubject.state, &proxySubject.bpm, &proxySubject.age)
	if err != nil {
		fmt.Println(err)
		infoLog.Println(err)
		return err
	}
	infoLog.Printf("Найдена строка %v\n", proxySubject)
	queryPrep = fmt.Sprintf("update SUBJECTS SET state=\"%s\" where id=\"%s\"", status, subjId)
	db.Exec(queryPrep)
	//query := db.QueryRow(queryPrep)
	if err != nil {
		infoLog.Println("Проблемы с базой данных")
		infoLog.Println(err)
		//infoLog.Println(query)
		db.Close()
		return err
	}
	db.Close()
	return nil
}
func dataInit() error {
	db, err := sql.Open("sqlite3", subjectsStoragePath)
	if err != nil {
		return err
	}
	rows, err := db.Query("select * from SUBJECTS")
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		storedSubject := subject{}
		err := rows.Scan(&storedSubject.id, &storedSubject.name, &storedSubject.state, &storedSubject.bpm, &storedSubject.age)
		if err != nil {
			fmt.Println(err)
			infoLog.Println(err)
			return err
		}
		fmt.Printf("Прочитано %s\n", storedSubject.id)
		infoLog.Printf("Прочитано %s\n", storedSubject.id)
		subjects = append(subjects, storedSubject)
	}
	fmt.Print(subjects[0])
	err = db.Close()
	if err != nil {
		return err
	}
	return nil
}

func initializeWatchDog() {
	err := logInit()
	if err != nil {
		log.Fatal("ОШИБКА ЛОГГЕРА")
		return
	}
	err = dataInit()
	if err != nil {
		log.Printf("НЕ НАЙДЕНА БАЗА ДАННЫХ")
		log.Fatal(err)
		return
	}
	err = initServerCode()
	if err != nil {
		log.Println("НЕ УДАЛОСЬ СГЕНЕРИРОВАТЬ ИЛИ ПОЛУЧИТЬ КОД СЕРВЕРА")
		log.Fatal(err)
		return
	}
	err = httpServerInit()
	if err != nil {
		log.Printf("НЕ УДАЛОСЬ ЗАПУСТИТЬ СЕРВЕР НА ПОРТ %v", 8090)
		log.Fatal(err)
		return
	}

}
func initServerCode() error {
	serverCodeBytes, serverCodeFileErr := os.ReadFile(SERVERCODE_PATH)
	if serverCodeFileErr != nil {
		serverCodeFile, err := os.Create(SERVERCODE_PATH)
		if err != nil {
			log.Println("Ошибка генерации или получения кода сервера")
			return err
		}
		serverCode, err = generateServerKey()
		if err != nil {
			log.Println("Ошибка генерации или получения кода сервера")
			return err
		}
		err = ioutil.WriteFile(serverCodeFile.Name(), []byte(serverCode), 0666)
		if err != nil {
			log.Println("Ошибка генерации или получения кода сервера")
			return err
		}
	} else {
		serverCode = string(serverCodeBytes)
	}
	return nil
}
func watchDogFunction() {

}
func main() {
	initializeWatchDog()
}
