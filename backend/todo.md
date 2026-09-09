should log this
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		return api.CreateHypothesis500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}
