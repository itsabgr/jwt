/*
 * Copyright 2018 The NATS Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package jwt

import (
	"fmt"
	"testing"
	"time"

	"github.com/itsabgr/nats-nkeys"
)

func TestNewOperatorClaims(t *testing.T) {
	ckp := createOperatorNKey(t)

	uc := NewOperatorClaims(publicKey(ckp, t))
	uc.Expires = time.Now().Add(time.Duration(time.Hour)).Unix()
	uJwt := encode(uc, ckp, t)

	uc2, err := DecodeOperatorClaims(uJwt)
	if err != nil {
		t.Fatal("failed to decode", err)
	}

	AssertEquals(uc.String(), uc2.String(), t)

	AssertEquals(uc.Claims() != nil, true, t)
	AssertEquals(uc.Payload() != nil, true, t)
}

func TestOperatorSubjects(t *testing.T) {
	type kpInputs struct {
		name string
		kp   nkeys.KeyPair
		ok   bool
	}

	inputs := []kpInputs{
		{"account", createAccountNKey(t), false},
		{"cluster", createClusterNKey(t), false},
		{"operator", createOperatorNKey(t), true},
		{"server", createServerNKey(t), false},
		{"user", createUserNKey(t), false},
	}

	for _, i := range inputs {
		c := NewOperatorClaims(publicKey(i.kp, t))
		_, err := c.Encode(createOperatorNKey(t))
		if i.ok && err != nil {
			t.Fatal(fmt.Sprintf("unexpected error for %q: %v", i.name, err))
		}
		if !i.ok && err == nil {
			t.Logf("should have failed to encode server with with %q subject", i.name)
			t.Fail()
		}
	}
}

func TestInvalidOperatorClaimIssuer(t *testing.T) {
	akp := createOperatorNKey(t)
	ac := NewOperatorClaims(publicKey(akp, t))
	ac.Expires = time.Now().Add(time.Duration(time.Hour)).Unix()
	aJwt := encode(ac, akp, t)

	temp, err := DecodeGeneric(aJwt)
	if err != nil {
		t.Fatal("failed to decode", err)
	}

	type kpInputs struct {
		name string
		kp   nkeys.KeyPair
		ok   bool
	}

	inputs := []kpInputs{
		{"account", createAccountNKey(t), false},
		{"user", createUserNKey(t), false},
		{"operator", createOperatorNKey(t), true},
		{"server", createServerNKey(t), false},
		{"cluster", createClusterNKey(t), false},
	}

	for _, i := range inputs {
		bad := encode(temp, i.kp, t)
		_, err = DecodeOperatorClaims(bad)
		if i.ok && err != nil {
			t.Fatal(fmt.Sprintf("unexpected error for %q: %v", i.name, err))
		}
		if !i.ok && err == nil {
			t.Logf("should have failed to decode account signed by %q", i.name)
			t.Fail()
		}
	}
}

func TestNewNilOperatorClaims(t *testing.T) {
	v := NewOperatorClaims("")
	if v != nil {
		t.Fatal("expected nil user claim")
	}
}

func TestOperatorType(t *testing.T) {
	c := NewOperatorClaims(publicKey(createOperatorNKey(t), t))
	s := encode(c, createOperatorNKey(t), t)
	u, err := DecodeOperatorClaims(s)
	if err != nil {
		t.Fatalf("failed to decode operator claim: %v", err)
	}

	if OperatorClaim != u.Type {
		t.Fatalf("type is unexpected %q (wanted operator)", u.Type)
	}

}

func TestSigningKeyValidation(t *testing.T) {
	ckp := createOperatorNKey(t)
	ckp2 := createOperatorNKey(t)

	uc := NewOperatorClaims(publicKey(ckp, t))
	uc.Expires = time.Now().Add(time.Duration(time.Hour)).Unix()
	uc.AddSigningKey(publicKey(ckp2, t))
	uJwt := encode(uc, ckp, t)

	uc2, err := DecodeOperatorClaims(uJwt)
	if err != nil {
		t.Fatal("failed to decode", err)
	}

	AssertEquals(len(uc2.SigningKeys), 1, t)
	AssertEquals(uc2.SigningKeys[0] == publicKey(ckp2, t), true, t)

	vr := &ValidationResults{}
	uc.Validate(vr)

	if len(vr.Issues) != 0 {
		t.Fatal("valid operator key should have no validation issues")
	}

	uc.AddSigningKey("") // add an invalid one

	vr = &ValidationResults{}
	uc.Validate(vr)
	if len(vr.Issues) != 0 {
		t.Fatal("should not be able to add empty values")
	}
}

func TestSignedBy(t *testing.T) {
	ckp := createOperatorNKey(t)
	ckp2 := createOperatorNKey(t)

	uc := NewOperatorClaims(publicKey(ckp, t))
	uc2 := NewOperatorClaims(publicKey(ckp2, t))

	akp := createAccountNKey(t)
	ac := NewAccountClaims(publicKey(akp, t))
	enc, err := ac.Encode(ckp) // sign with the operator key
	if err != nil {
		t.Fatal("failed to encode", err)
	}
	ac, err = DecodeAccountClaims(enc)
	if err != nil {
		t.Fatal("failed to decode", err)
	}

	AssertEquals(uc.DidSign(ac), true, t)
	AssertEquals(uc2.DidSign(ac), false, t)

	enc, err = ac.Encode(ckp2) // sign with the other operator key
	if err != nil {
		t.Fatal("failed to encode", err)
	}
	ac, err = DecodeAccountClaims(enc)
	if err != nil {
		t.Fatal("failed to decode", err)
	}

	AssertEquals(uc.DidSign(ac), false, t) // no signing key
	AssertEquals(uc2.DidSign(ac), true, t) // actual key
	uc.AddSigningKey(publicKey(ckp2, t))
	AssertEquals(uc.DidSign(ac), true, t) // signing key

	clusterKey := createClusterNKey(t)
	clusterClaims := NewClusterClaims(publicKey(clusterKey, t))
	enc, err = clusterClaims.Encode(ckp2) // sign with the operator key
	if err != nil {
		t.Fatal("failed to encode", err)
	}
	clusterClaims, err = DecodeClusterClaims(enc)
	if err != nil {
		t.Fatal("failed to decode", err)
	}

	AssertEquals(uc.DidSign(clusterClaims), true, t)  // signing key
	AssertEquals(uc2.DidSign(clusterClaims), true, t) // actual key
}

func testAccountWithAccountServerURL(t *testing.T, u string) error {
	kp := createOperatorNKey(t)
	pk := publicKey(kp, t)
	oc := NewOperatorClaims(pk)
	oc.AccountServerURL = u

	s, err := oc.Encode(kp)
	if err != nil {
		return err
	}
	oc, err = DecodeOperatorClaims(s)
	if err != nil {
		t.Fatal(err)
	}
	AssertEquals(oc.AccountServerURL, u, t)
	vr := ValidationResults{}
	oc.Validate(&vr)
	if !vr.IsEmpty() {
		errs := vr.Errors()
		return errs[0]
	}
	return nil
}

func Test_SystemAccount(t *testing.T) {
	operatorWithSystemAcc := func(t *testing.T, u string) error {
		kp := createOperatorNKey(t)
		pk := publicKey(kp, t)
		oc := NewOperatorClaims(pk)
		oc.SystemAccount = u
		s, err := oc.Encode(kp)
		if err != nil {
			return err
		}
		oc, err = DecodeOperatorClaims(s)
		if err != nil {
			t.Fatal(err)
		}
		AssertEquals(oc.SystemAccount, u, t)
		vr := ValidationResults{}
		oc.Validate(&vr)
		if !vr.IsEmpty() {
			return fmt.Errorf("%s", vr.Errors()[0])
		}
		return nil
	}
	var asuTests = []struct {
		accKey     string
		shouldFail bool
	}{
		{"", false},
		{"x", true},
		//		{"ADZ547B24WHPLWOK7TMLNBSA7FQFXR6UM2NZ4HHNIB7RDFVZQFOZ4GQQ", false},
		//		{"ADZ547B24WHPLWOK7TMLNBSA7FQFXR6UM2NZ4HHNIB7RDFVZQFOZ4777", true},
	}
	for i, tt := range asuTests {
		err := operatorWithSystemAcc(t, tt.accKey)
		if err != nil && tt.shouldFail == false {
			t.Fatalf("expected not to fail: %v", err)
		} else if err == nil && tt.shouldFail {
			t.Fatalf("test %s expected to fail but didn't", asuTests[i].accKey)
		}
	}
}

func Test_AccountServerURL(t *testing.T) {
	var asuTests = []struct {
		u          string
		shouldFail bool
	}{
		{"", false},
		{"HTTP://foo.bar.com", false},
		{"http://foo.bar.com/foo/bar", false},
		{"http://user:pass@foo.bar.com/foo/bar", false},
		//		{"https://foo.bar.com", false},
		{"nats://foo.bar.com", false},
		{"/hello", true},
	}

	for i, tt := range asuTests {
		err := testAccountWithAccountServerURL(t, tt.u)
		if err != nil && tt.shouldFail == false {
			t.Fatalf("expected not to fail: %v", err)
		} else if err == nil && tt.shouldFail {
			t.Fatalf("test %s expected to fail but didn't", asuTests[i].u)
		}
	}
}

func testOperatorWithOperatorServiceURL(t *testing.T, u string) error {
	kp := createOperatorNKey(t)
	pk := publicKey(kp, t)
	oc := NewOperatorClaims(pk)
	oc.OperatorServiceURLs.Add(u)

	s, err := oc.Encode(kp)
	if err != nil {
		return err
	}
	oc, err = DecodeOperatorClaims(s)
	if err != nil {
		t.Fatal(err)
	}
	if u != "" {
		AssertEquals(oc.OperatorServiceURLs[0], u, t)
	}
	vr := ValidationResults{}
	oc.Validate(&vr)
	if !vr.IsEmpty() {
		errs := vr.Errors()
		return errs[0]
	}
	return nil
}

func Test_OperatorServiceURL(t *testing.T) {
	var asuTests = []struct {
		u          string
		shouldFail bool
	}{
		{"", false},
		{"HTTP://foo.bar.com", true},
		{"http://foo.bar.com/foo/bar", true},
		{"nats://user:pass@foo.bar.com", true},
		{"NATS://user:pass@foo.bar.com", true},
		{"NATS://user@foo.bar.com", true},
		{"nats://foo.bar.com/path", true},
		{"tls://foo.bar.com/path", true},
		{"/hello", true},
		{"NATS://foo.bar.com", false},
		{"TLS://foo.bar.com", false},
		{"nats://foo.bar.com", false},
		{"tls://foo.bar.com", false},
	}

	for i, tt := range asuTests {
		err := testOperatorWithOperatorServiceURL(t, tt.u)
		if err != nil && tt.shouldFail == false {
			t.Fatalf("expected not to fail: %v", err)
		} else if err == nil && tt.shouldFail {
			t.Fatalf("test %s expected to fail but didn't", asuTests[i].u)
		}
	}

	// now test all of them in a single jwt
	kp := createOperatorNKey(t)
	pk := publicKey(kp, t)
	oc := NewOperatorClaims(pk)

	encoded := 0
	shouldFail := 0
	for _, v := range asuTests {
		oc.OperatorServiceURLs.Add(v.u)
		// list won't encode empty strings
		if v.u != "" {
			encoded++
		}
		if v.shouldFail {
			shouldFail++
		}
	}

	s, err := oc.Encode(kp)
	if err != nil {
		t.Fatal(err)
	}
	oc, err = DecodeOperatorClaims(s)
	if err != nil {
		t.Fatal(err)
	}

	AssertEquals(len(oc.OperatorServiceURLs), encoded, t)

	vr := ValidationResults{}
	oc.Validate(&vr)
	if vr.IsEmpty() {
		t.Fatal("should have had errors")
	}

	errs := vr.Errors()
	AssertEquals(len(errs), shouldFail, t)
}

func Test_ForwardCompatibility(t *testing.T) {
	newOp := `eyJ0eXAiOiJqd3QiLCJhbGciOiJlZDI1NTE5LW5rZXkifQ.eyJqdGkiOiJTSUYyR0ZRSEhWWUtDQlZYRklYUURYV1FCQUcyWEw3SVZLVVJZT0ZTWlhVT0tTRUpLWDdBIiwiaWF0IjoxNTkwNTI0NTAwLCJpc3MiOiJPQlQ2REtGSzQ2STM3TjdCUkwyUkpMVVJLWUdSQTZBWVJQREFISFFFQUFBR05ZWExNR1JEUEtMQyIsInN1YiI6Ik9CVDZES0ZLNDZJMzdON0JSTDJSSkxVUktZR1JBNkFZUlBEQUhIUUVBQUFHTllYTE1HUkRQS0xDIiwibmF0cyI6eyJ0YWdzIjpbIm9uZSIsInR3byIsInRocmVlIl0sInR5cGUiOiJvcGVyYXRvciIsInZlcnNpb24iOjJ9fQ.u6JFiISIh2o-CWxktfEw3binmCLhLaFVMyuIa2HNo_x_6EGWVPVICVWc_MOLFS-6Nm17Cj4SmOh3zUtlTRkfDA`
	if _, err := DecodeOperatorClaims(newOp); err == nil {
		t.Fatal("Expected error")
	} else if err.Error() != `more recent jwt version` {
		t.Fatal("Expected different error, got: ", err)
	}
}
