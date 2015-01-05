package influxdb

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/bmizerany/pat"
	"github.com/influxdb/influxdb/influxql"
)

// TODO: Standard response headers (see: HeaderHandler)
// TODO: Compression (see: CompressionHeaderHandler)

// TODO: Check HTTP response codes: 400, 401, 403, 409.

// getUsernameAndPassword returns the username and password encoded in
// a request. The credentials may be present as URL query params, or as
// a Basic Authentication header.
func getUsernameAndPassword(r *http.Request) (string, string, error) {
	q := r.URL.Query()
	username, password := q.Get("u"), q.Get("p")
	if username != "" && password != "" {
		return username, password, nil
	}
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", "", nil
	}
	fields := strings.Split(auth, " ")
	if len(fields) != 2 {
		return "", "", fmt.Errorf("invalid Basic Authentication header")
	}
	bs, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid Base64 encoding")
	}
	fields = strings.Split(string(bs), ":")
	if len(fields) != 2 {
		return "", "", fmt.Errorf("invalid Basic Authentication value")
	}
	return fields[0], fields[1], nil
}

// Handler represents an HTTP handler for the InfluxDB server.
type Handler struct {
	server *Server
	mux    *pat.PatternServeMux

	// Whether endpoints require authentication.
	AuthenticationEnabled bool

	// The InfluxDB verion returned by the HTTP response header.
	Version string
}

// NewHandler returns a new instance of Handler.
func NewHandler(s *Server) *Handler {
	h := &Handler{
		server: s,
		mux:    pat.New(),
	}

	// Authentication route
	h.mux.Get("/authenticate", http.HandlerFunc(h.serveAuthenticate))

	// User routes.
	h.mux.Get("/users", h.makeAuthenticationHandler(h.serveUsers))
	h.mux.Post("/users", http.HandlerFunc(h.serveCreateUser)) // Non-standard authentication
	h.mux.Put("/users/:user", h.makeAuthenticationHandler(h.serveUpdateUser))
	h.mux.Del("/users/:user", h.makeAuthenticationHandler(h.serveDeleteUser))

	// Database routes
	h.mux.Get("/db", h.makeAuthenticationHandler(h.serveDatabases))
	h.mux.Post("/db", h.makeAuthenticationHandler(h.serveCreateDatabase))
	h.mux.Del("/db/:name", h.makeAuthenticationHandler(h.serveDeleteDatabase))

	// Series routes.
	h.mux.Get("/db/:db/series", h.makeAuthenticationHandler(h.serveQuery))
	h.mux.Post("/db/:db/series", h.makeAuthenticationHandler(h.serveWriteSeries))

	// Shard routes.
	h.mux.Get("/db/:db/shards", h.makeAuthenticationHandler(h.serveShards))
	h.mux.Del("/db/:db/shards/:id", h.makeAuthenticationHandler(h.serveDeleteShard))

	// Retention policy routes.
	h.mux.Get("/db/:db/retention_policies", h.makeAuthenticationHandler(h.serveRetentionPolicies))
	h.mux.Post("/db/:db/retention_policies", h.makeAuthenticationHandler(h.serveCreateRetentionPolicy))
	h.mux.Put("/db/:db/retention_policies/:name", h.makeAuthenticationHandler(h.serveUpdateRetentionPolicy))
	h.mux.Del("/db/:db/retention_policies/:name", h.makeAuthenticationHandler(h.serveDeleteRetentionPolicy))

	// Data node routes.
	h.mux.Get("/data_nodes", h.makeAuthenticationHandler(h.serveDataNodes))
	h.mux.Post("/data_nodes", h.makeAuthenticationHandler(h.serveCreateDataNode))
	h.mux.Del("/data_nodes/:id", h.makeAuthenticationHandler(h.serveDeleteDataNode))

	// Utilities
	h.mux.Get("/ping", h.makeAuthenticationHandler(h.servePing))

	return h
}

// ServeHTTP responds to HTTP request to the handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Max-Age", "2592000")
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
	w.Header().Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	w.Header().Add("X-Influxdb-Version", h.Version)

	// If this is a CORS OPTIONS request then send back okie-dokie.
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Otherwise handle it via pat.
	h.mux.ServeHTTP(w, r)
}

// makeAuthenticationHandler takes a custom handler and returns a standard handler, ensuring that
// the system's standard authentication policies have been applied before the custom handler is called.
//
// The standard policy is if authentication is disabled, all operations are allowed and no user credentials
// are required. If authentication is enabled, valid user credentials must be supplied.
func (h *Handler) makeAuthenticationHandler(fn func(http.ResponseWriter, *http.Request, *User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var user *User
		if h.AuthenticationEnabled {
			username, password, err := getUsernameAndPassword(r)
			if err != nil {
				h.error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			user, err = h.server.AuthenticatedUser(username, password)
			if err != nil {
				h.error(w, err.Error(), http.StatusUnauthorized)
				return
			}
		}
		fn(w, r, user)
	}
}

// serveQuery parses an incoming query and returns the results.
func (h *Handler) serveQuery(w http.ResponseWriter, r *http.Request, u *User) {
	// TODO: Authentication.

	// Parse query from query string.
	urlQry := r.URL.Query()
	_, err := influxql.NewParser(strings.NewReader(urlQry.Get("q"))).ParseQuery()
	if err != nil {
		h.error(w, "parse error: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Retrieve database from server.
	/*
		db := h.server.Database(urlQry.Get(":db"))
		if db == nil {
			h.error(w, ErrDatabaseNotFound.Error(), http.StatusNotFound)
			return
		}
	*/

	// Parse the time precision from the query params.
	/*
		precision, err := parseTimePrecision(urlQry.Get("time_precision"))
		if err != nil {
			h.error(w, err.Error(), http.StatusBadRequest)
			return
		}
	*/

	// Execute query against the database.
	/*
		if err := db.ExecuteQuery(q); err != nil {
			h.error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	*/
}

// serveWriteSeries receives incoming series data and writes it to the database.
func (h *Handler) serveWriteSeries(w http.ResponseWriter, r *http.Request, u *User) {
	// TODO: Authentication.

	/* TEMPORARILY REMOVED FOR PROTOBUFS.
	// Retrieve database from server.
	db := h.server.Database(r.URL.Query().Get(":db"))
	if db == nil {
		h.error(w, ErrDatabaseNotFound.Error(), http.StatusNotFound)
		return
	}

	// Parse time precision from query parameters.
	precision, err := parseTimePrecision(r.URL.Query().Get("time_precision"))
	if err != nil {
		h.error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Setup HTTP request reader. Wrap in a gzip reader if encoding set in header.
	reader := r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		if reader, err = gzip.NewReader(r.Body); err != nil {
			h.error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Decode series from reader.
	ss := []*serializedSeries{}
	dec := json.NewDecoder(reader)
	dec.UseNumber()
	if err := dec.Decode(&ss); err != nil {
		h.error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Convert the wire format to the internal representation of the time series.
	series, err := serializedSeriesSlice(ss).series(precision)
	if err != nil {
		h.error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Write series data to the database.
	// TODO: Allow multiple series written to DB at once.
	for _, s := range series {
		if err := db.WriteSeries(s); err != nil {
			h.error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	*/
}

// serveDatabases returns a list of all databases on the server.
func (h *Handler) serveDatabases(w http.ResponseWriter, r *http.Request, u *User) {

	// Retrieve databases from the server.
	databases := h.server.Databases()

	// JSON encode databases to the response.
	w.Header().Add("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(databases)
}

// serveCreateDatabase creates a new database on the server.
func (h *Handler) serveCreateDatabase(w http.ResponseWriter, r *http.Request, u *User) {
	var req struct {
		Name string `json:"name"`
	}

	// Decode the request from the body.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		h.error(w, err.Error(), http.StatusBadRequest)
		return
	} else if req.Name == "" {
		h.error(w, ErrDatabaseNameRequired.Error(), http.StatusBadRequest)
		return
	}

	// Create the database.
	if err := h.server.CreateDatabase(req.Name); err == ErrDatabaseExists {
		h.error(w, err.Error(), http.StatusConflict)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// serveDeleteDatabase deletes an existing database on the server.
func (h *Handler) serveDeleteDatabase(w http.ResponseWriter, r *http.Request, u *User) {
	name := r.URL.Query().Get(":name")
	if err := h.server.DeleteDatabase(name); err == ErrDatabaseNotFound {
		h.error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// serveAuthenticate authenticates a user.
func (h *Handler) serveAuthenticate(w http.ResponseWriter, r *http.Request) {}

// serveUsers returns data about a single user.
func (h *Handler) serveUsers(w http.ResponseWriter, r *http.Request, u *User) {

	// Generate a list of objects for encoding to the API.
	a := make([]*userJSON, 0)
	for _, u := range h.server.Users() {
		a = append(a, &userJSON{
			Name:  u.Name,
			Admin: u.Admin,
		})
	}

	w.Header().Add("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(a)
}

type userJSON struct {
	Name     string `json:"name"`
	Password string `json:"password,omitempty"`
	Admin    bool   `json:"admin,omitempty"`
}

// serveCreateUser creates a new user.
func (h *Handler) serveCreateUser(w http.ResponseWriter, r *http.Request) {
	// Read in user from request body.
	var newUser userJSON
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		h.error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Creating a User involves a non-standard authentication policy. Iff no Admin
	// already exists, and the used being created will be an admin, no authorization
	// is required.
	if h.AuthenticationEnabled && (h.server.AdminUserExists() || !newUser.Admin) {
		username, password, err := getUsernameAndPassword(r)
		if err != nil {
			h.error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		_, err = h.server.AuthenticatedUser(username, password)
		if err != nil {
			h.error(w, err.Error(), http.StatusUnauthorized)
			return
		}
	}

	// Create the user.
	if err := h.server.CreateUser(newUser.Name, newUser.Password, newUser.Admin); err == ErrUserExists {
		h.error(w, err.Error(), http.StatusConflict)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// serveUpdateUser updates an existing user.
func (h *Handler) serveUpdateUser(w http.ResponseWriter, r *http.Request, u *User) {
	// Read in user from request body.
	var user userJSON
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		h.error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create the user.
	if err := h.server.UpdateUser(r.URL.Query().Get(":user"), user.Password); err == ErrUserNotFound {
		h.error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// serveDeleteUser removes an existing user.
func (h *Handler) serveDeleteUser(w http.ResponseWriter, r *http.Request, u *User) {
	// Delete the user.
	if err := h.server.DeleteUser(r.URL.Query().Get(":user")); err == ErrUserNotFound {
		h.error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// servePing returns a simple response to let the client know the server is running.
func (h *Handler) servePing(w http.ResponseWriter, r *http.Request, u *User) {}

// serveShards returns a list of shards.
func (h *Handler) serveShards(w http.ResponseWriter, r *http.Request, u *User) {
	q := r.URL.Query()

	// Retrieves shards for the database.
	shards, err := h.server.Shards(q.Get(":db"))
	if err == ErrDatabaseNotFound {
		h.error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write data to the response.
	w.Header().Add("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(shards)
}

// serveDeleteShard removes an existing shard.
func (h *Handler) serveDeleteShard(w http.ResponseWriter, r *http.Request, u *User) {}

// serveRetentionPolicies returns a list of retention policys.
func (h *Handler) serveRetentionPolicies(w http.ResponseWriter, r *http.Request, u *User) {
	// Retrieve policies by database.
	policies, err := h.server.RetentionPolicies(r.URL.Query().Get(":db"))
	if err == ErrDatabaseNotFound {
		h.error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write data to response body.
	w.Header().Add("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(policies)
}

// serveCreateRetentionPolicy creates a new retention policy.
func (h *Handler) serveCreateRetentionPolicy(w http.ResponseWriter, r *http.Request, u *User) {
	// Decode the policy from the body.
	var policy RetentionPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		h.error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create the retention policy.
	if err := h.server.CreateRetentionPolicy(r.URL.Query().Get(":db"), &policy); err == ErrDatabaseNotFound {
		h.error(w, err.Error(), http.StatusNotFound)
		return
	} else if err == ErrRetentionPolicyExists {
		h.error(w, err.Error(), http.StatusConflict)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// serveUpdateRetentionPolicy updates an existing retention policy.
func (h *Handler) serveUpdateRetentionPolicy(w http.ResponseWriter, r *http.Request, u *User) {
	q := r.URL.Query()
	db, name := q.Get(":db"), q.Get(":name")

	// Decode the new policy values from the body.
	var policy RetentionPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		h.error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update the retention policy.
	if err := h.server.UpdateRetentionPolicy(db, name, &policy); err == ErrDatabaseNotFound || err == ErrRetentionPolicyNotFound {
		h.error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// serveDeleteRetentionPolicy removes an existing retention policy.
func (h *Handler) serveDeleteRetentionPolicy(w http.ResponseWriter, r *http.Request, u *User) {
	q := r.URL.Query()
	db, name := q.Get(":db"), q.Get(":name")

	// Delete the retention policy.
	if err := h.server.DeleteRetentionPolicy(db, name); err == ErrDatabaseNotFound || err == ErrRetentionPolicyNotFound {
		h.error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// serveDataNodes returns a list of all data nodes in the cluster.
func (h *Handler) serveDataNodes(w http.ResponseWriter, r *http.Request, u *User) {
	// Generate a list of objects for encoding to the API.
	a := make([]*dataNodeJSON, 0)
	for _, n := range h.server.DataNodes() {
		a = append(a, &dataNodeJSON{
			ID:  n.ID,
			URL: n.URL.String(),
		})
	}

	w.Header().Add("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(a)
}

// serveCreateDataNode creates a new data node in the cluster.
func (h *Handler) serveCreateDataNode(w http.ResponseWriter, r *http.Request, u *User) {
	// Read in data node from request body.
	var n dataNodeJSON
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		h.error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Parse the URL.
	url, err := url.Parse(n.URL)
	if err != nil {
		h.error(w, "invalid data node url", http.StatusBadRequest)
		return
	}

	// Create the data node.
	if err := h.server.CreateDataNode(url); err == ErrDataNodeExists {
		h.error(w, err.Error(), http.StatusConflict)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write new node back to client.
	node := h.server.DataNodeByURL(url)
	w.WriteHeader(http.StatusCreated)
	w.Header().Add("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(&dataNodeJSON{ID: node.ID, URL: node.URL.String()})
}

// serveDeleteDataNode removes an existing node.
func (h *Handler) serveDeleteDataNode(w http.ResponseWriter, r *http.Request, u *User) {
	// Parse node id.
	nodeID, err := strconv.ParseUint(r.URL.Query().Get(":id"), 10, 64)
	if err != nil {
		h.error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	// Delete the node.
	if err := h.server.DeleteDataNode(nodeID); err == ErrDataNodeNotFound {
		h.error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		h.error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type dataNodeJSON struct {
	ID  uint64 `json:"id"`
	URL string `json:"url"`
}

// error returns an error to the client in a standard format.
func (h *Handler) error(w http.ResponseWriter, error string, code int) {
	// TODO: Return error as JSON.
	http.Error(w, error, code)
}
