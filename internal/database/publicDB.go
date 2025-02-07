package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v4/pgxpool"
)

func (repo *MyRepository) RegReportSQL(ctx context.Context, rw http.ResponseWriter, rep *pgxpool.Pool, order_id int) (err error) {
	repo_id, err := rep.Query(ctx,
		`INSERT INTO Public.report (order_id)
		     VALUES ($1)
		 RETURNING id;`,

		order_id,
	)
	errorr(err)

	var repo_id_int int
	for repo_id.Next() {
		err := repo_id.Scan(
			&repo_id_int,
		)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}
	type Response struct {
		Status  string
		Data    int
		Message string
	}

	if err != nil || repo_id_int == 0 {
		response := Response{
			Status:  "success",
			Data:    0,
			Message: "The user has been successfully registered",
		}

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)

		return
	}
	response := Response{
		Status:  "success",
		Data:    repo_id_int,
		Message: "The user has been successfully registered",
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(response)

	return
}
