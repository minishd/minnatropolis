package api

type registerRequest struct {
	Username string
	Password string
}

type registerResponse struct {
}

func handleRegister(req registerRequest) (res registerResponse, err error) {
	return
}

type loginRequest struct {
	Username string
	Password string
}

type loginResponse struct {
	Token string
}

func handleLogin(req loginRequest) (res loginResponse, err error) {

	return
}
