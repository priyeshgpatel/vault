package http

import (
	"encoding/hex"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/vault/helper/pgpkeys"
	"github.com/hashicorp/vault/vault"
)

func TestSysInit_get(t *testing.T) {
	core := vault.TestCore(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()

	{
		// Pre-init
		resp, err := http.Get(addr + "/v1/sys/init")
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		var actual map[string]interface{}
		expected := map[string]interface{}{
			"initialized": false,
		}
		testResponseStatus(t, resp, 200)
		testResponseBody(t, resp, &actual)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("bad: %#v", actual)
		}
	}

	vault.TestCoreInit(t, core)

	{
		// Post-init
		resp, err := http.Get(addr + "/v1/sys/init")
		if err != nil {
			t.Fatalf("err: %s", err)
		}

		var actual map[string]interface{}
		expected := map[string]interface{}{
			"initialized": true,
		}
		testResponseStatus(t, resp, 200)
		testResponseBody(t, resp, &actual)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("bad: %#v", actual)
		}
	}
}

// Test to check if the API errors out when pgp and wrap shares are supplied
func TestSysInit_pgpAndWrap(t *testing.T) {
	core := vault.TestCore(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()

	resp := testHttpPut(t, "", addr+"/v1/sys/init", map[string]interface{}{
		"secret_shares":    1,
		"secret_threshold": 1,
		"pgp_keys":         []string{pgpkeys.TestPubKey1},
		"wrap_shares":      true,
	})
	testResponseStatus(t, resp, 400)
}

// Test to check if the API errors out when wrong number of PGP keys are
// supplied
func TestSysInit_pgpKeysEntries(t *testing.T) {
	core := vault.TestCore(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()

	resp := testHttpPut(t, "", addr+"/v1/sys/init", map[string]interface{}{
		"secret_shares":    5,
		"secret_threshold": 3,
		"pgp_keys":         []string{"pgpkey1"},
	})
	testResponseStatus(t, resp, 400)
}

// Test to check if the API errors out when wrong number of PGP keys are
// supplied for recovery config
func TestSysInit_pgpKeysEntriesForRecovery(t *testing.T) {
	core := vault.TestCoreNewSeal(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()

	resp := testHttpPut(t, "", addr+"/v1/sys/init", map[string]interface{}{
		"secret_shares":      1,
		"secret_threshold":   1,
		"stored_shares":      1,
		"recovery_shares":    5,
		"recovery_threshold": 3,
		"recovery_pgp_keys":  []string{"pgpkey1"},
	})
	testResponseStatus(t, resp, 400)
}

func TestSysInit_put(t *testing.T) {
	core := vault.TestCore(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()

	resp := testHttpPut(t, "", addr+"/v1/sys/init", map[string]interface{}{
		"secret_shares":    5,
		"secret_threshold": 3,
	})

	var actual map[string]interface{}
	testResponseStatus(t, resp, 200)
	testResponseBody(t, resp, &actual)
	keysRaw, ok := actual["keys"]
	if !ok {
		t.Fatalf("no keys: %#v", actual)
	}

	root, ok := actual["root_token"]
	if !ok {
		t.Fatal("no root token")
	}

	if err := core.Seal(root.(string)); err != nil {
		t.Fatalf("err: %s", err)
	}

	for _, key := range keysRaw.([]interface{}) {
		keySlice, err := hex.DecodeString(key.(string))
		if err != nil {
			t.Fatalf("bad: %s", err)
		}

		if _, err := core.Unseal(keySlice); err != nil {
			t.Fatalf("bad: %s", err)
		}
	}

	seal, err := core.Sealed()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if seal {
		t.Fatal("should not be sealed")
	}
}

func TestSysInit_wrap(t *testing.T) {
	core := vault.TestCore(t)
	ln, addr := TestServer(t, core)
	defer ln.Close()

	resp := testHttpPut(t, "", addr+"/v1/sys/init", map[string]interface{}{
		"secret_shares":    5,
		"secret_threshold": 3,
		"wrap_shares":      true,
	})

	var actual map[string]interface{}
	testResponseStatus(t, resp, 200)
	testResponseBody(t, resp, &actual)
	keysRaw, ok := actual["keys"]
	if !ok {
		t.Fatalf("no keys: %#v", actual)
	}

	root, ok := actual["root_token"]
	if !ok {
		t.Fatal("no root token")
	}

	if root != "" {
		t.Fatalf("root token should be empty")
	}

	for _, key := range keysRaw.([]interface{}) {
		if strings.Count(key.(string), ".") != 2 {
			t.Fatalf("expected JWT token, got: %s", key.(string))
		}
	}

	seal, err := core.Sealed()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if seal {
		t.Fatal("should not be sealed")
	}
}
