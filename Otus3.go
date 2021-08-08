package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type User struct {
	Id        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type Error struct {
	code    int32
	message string
}

func main() {
	initMetrics()
	r := mux.NewRouter()
	r.HandleFunc("/user", addUser).Methods("POST")
	r.HandleFunc("/user/{id}", getUser).Methods("GET")
	r.HandleFunc("/user/{id}", updateUser).Methods("PUT")
	r.HandleFunc("/user/{id}", deleteUser).Methods("DELETE")
	r.PathPrefix("/metrics").Handler(promhttp.Handler())
	http.Handle("/", r)

	fmt.Println("Start listening on 8000")
	http.ListenAndServe(":8000", nil)
}

func initMetrics() {
	prometheus.MustRegister(RequestCountAdd)
	prometheus.MustRegister(RequestCountGet)
	prometheus.MustRegister(RequestCountPut)
	prometheus.MustRegister(RequestCountDelete)

	prometheus.MustRegister(ErrorAdd)
	prometheus.MustRegister(ErrorGet)
	prometheus.MustRegister(ErrorPut)
	prometheus.MustRegister(ErrorDelete)

	prometheus.MustRegister(LatencyAdd)
	prometheus.MustRegister(LatencyGet)
	prometheus.MustRegister(LatencyPut)
	prometheus.MustRegister(LatencyDelete)
}

func addUser(writer http.ResponseWriter, request *http.Request) {
	requestStart := time.Now()

	writer.Header().Set("Context-Type", "application/x-www-form-urlencoded")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Methods", "POST")
	writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	var user User

	err := json.NewDecoder(request.Body).Decode(&user)

	if err != nil {
		error := Error{
			code:    0,
			message: "Unable to decode the request body. ",
		}
		json.NewEncoder(writer).Encode(error)
		ErrorAdd.Inc()
		return
	}

	_ = insertUser(user)

	writer.WriteHeader(200)

	RequestCountAdd.Inc()
	requestTime := time.Since(requestStart).Seconds()
	log.Printf("requestTime %s", requestTime)
	LatencyAdd.Observe(requestTime)
}

func getUser(writer http.ResponseWriter, request *http.Request) {
	requestStart := time.Now()

	writer.Header().Set("Context-Type", "application/x-www-form-urlencoded")
	writer.Header().Set("Access-Control-Allow-Origin", "*")

	params := mux.Vars(request)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		error := Error{
			code:    0,
			message: "Unable to convert the string into int.",
		}
		json.NewEncoder(writer).Encode(error)
		ErrorGet.Inc()
		return
	}

	user, err := getUserFromDB(int64(id))

	if err != nil {
		error := Error{
			code:    0,
			message: "Unable to get user.",
		}
		json.NewEncoder(writer).Encode(error)
		ErrorGet.Inc()
		return
	}

	writer.WriteHeader(200)
	json.NewEncoder(writer).Encode(user)

	RequestCountGet.Inc()
	requestTime := time.Since(requestStart).Seconds()
	log.Printf("requestTime %s", requestTime)
	LatencyGet.Observe(requestTime)
}

func updateUser(writer http.ResponseWriter, request *http.Request) {
	requestStart := time.Now()

	writer.Header().Set("Content-Type", "application/x-www-form-urlencoded")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Methods", "PUT")
	writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	params := mux.Vars(request)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		error := Error{
			code:    0,
			message: "Unable to convert the string into int.",
		}
		json.NewEncoder(writer).Encode(error)
		ErrorPut.Inc()
		return
	}

	var user User

	err = json.NewDecoder(request.Body).Decode(&user)

	if err != nil {
		error := Error{
			code:    0,
			message: "Unable to decode the request body.",
		}
		json.NewEncoder(writer).Encode(error)
		ErrorPut.Inc()
		return
	}

	_ = updateUserInDB(int64(id), user)

	writer.WriteHeader(200)

	RequestCountPut.Inc()
	requestTime := time.Since(requestStart).Seconds()
	log.Printf("requestTime %s", requestTime)
	LatencyPut.Observe(requestTime)
}

func deleteUser(writer http.ResponseWriter, request *http.Request) {
	requestStart := time.Now()

	writer.Header().Set("Context-Type", "application/x-www-form-urlencoded")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Methods", "DELETE")
	writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	params := mux.Vars(request)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		error := Error{
			code:    0,
			message: "Unable to convert the string into int.",
		}
		json.NewEncoder(writer).Encode(error)
		ErrorDelete.Inc()
		return
	}

	_ = deleteUserFromDB(int64(id))

	writer.WriteHeader(204)

	RequestCountDelete.Inc()
	requestTime := time.Since(requestStart).Seconds()
	log.Printf("requestTime %s", requestTime)
	LatencyDelete.Observe(requestTime)
}

func insertUser(user User) int64 {
	db := createConnection()
	defer db.Close()

	sqlStatement := `INSERT INTO users (username, firstName, lastName, email, phone) VALUES ($1, $2, $3, $4, $5) RETURNING Id`

	var id int64

	err := db.QueryRow(sqlStatement, user.Username, user.FirstName, user.LastName, user.Email, user.Phone).Scan(&id)
	if err != nil {
		log.Fatalf("Unable to execute the query. %v", err)
	}

	fmt.Printf("Inserted a single record %v", id)
	return id
}

func getUserFromDB(id int64) (User, error) {
	db := createConnection()
	defer db.Close()

	var user User

	sqlStatement := `SELECT * FROM users WHERE id=$1`

	row := db.QueryRow(sqlStatement, id)

	err := row.Scan(&user.Id, &user.Username, &user.FirstName, &user.LastName, &user.Email, &user.Phone)

	switch err {
	case sql.ErrNoRows:
		fmt.Println("No rows were returned!")
		return user, nil
	case nil:
		return user, nil
	default:
		log.Fatalf("Unable to scan the row. %v", err)
	}

	return user, err
}

func updateUserInDB(id int64, user User) int64 {
	db := createConnection()
	defer db.Close()

	sqlStatement := `UPDATE users SET username=$2, firstName=$3, lastName=$4, email=$5, phone=$6 WHERE id=$1`

	res, err := db.Exec(sqlStatement, id, user.Username, user.FirstName, user.LastName, user.Email, &user.Phone)
	if err != nil {
		log.Fatalf("Unable to execute the query. %v", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Fatalf("Error while checking the affected rows. %v", err)
	}

	fmt.Printf("Total rows/record affected %v", rowsAffected)
	return rowsAffected
}

func deleteUserFromDB(id int64) int64 {
	db := createConnection()
	defer db.Close()

	sqlStatement := `DELETE FROM users WHERE id=$1`

	res, err := db.Exec(sqlStatement, id)
	if err != nil {
		log.Fatalf("Unable to execute the query. %v", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		log.Fatalf("Error while checking the affected rows. %v", err)
	}

	fmt.Printf("Total rows/record affected %v", rowsAffected)
	return rowsAffected
}

func createConnection() *sql.DB {
	psqlconn := os.Getenv("DATABASE_URI")
	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	fmt.Println("Successfully connected!")
	return db
}
