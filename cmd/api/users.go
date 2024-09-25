package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rupayan-ninety-eight/greenlight/internal/data"
	"github.com/rupayan-ninety-eight/greenlight/internal/validator"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 3*time.Second)
	defer cancel()

	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Users.Insert(ctx, user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		case errors.Is(err, context.DeadlineExceeded):
			app.timeoutExceededResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	app.background(func() {
		err = app.mailer.Send(user.Email, "user_welcome.tmpl.html", user)
		if err != nil {
			app.logger.Error(err.Error())
			return
		}
		app.logger.Info("welcome email sent to", "email", user.Email)
	})

	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
