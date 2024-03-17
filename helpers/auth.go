package helpers

import (
	"binchecker/credential"
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func LoadTLSCredentials() (credentials.TransportCredentials, error) {
	serverCert, err := tls.LoadX509KeyPair("cert/server-cert.pem", "cert/server-key.pem")
	if err != nil {
		return nil, err
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
	}
	return credentials.NewTLS(config), nil
}
func Verify(accessToken string) bool {
	_, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("err")
		}
		return []byte(credential.JWT_SECRET), nil
	})
	return err == nil
}

func StreamInterceptor(srv interface{}, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return status.Errorf(codes.Unauthenticated, codes.Unauthenticated.String())
	}
	values := md["authorization"]
	if len(values) > 0 {
		if !Verify(values[0]) {
			return status.Error(codes.PermissionDenied, codes.PermissionDenied.String())
		}
	} else {
		return status.Error(codes.PermissionDenied, codes.PermissionDenied.String())
	}
	return handler(srv, stream)
}

func GenerateToken(uuid string) (string, error) {
	claims := &jwt.MapClaims{
		"exp":  time.Now().Add(24 * 7 * time.Hour).Unix(),
		"uuid": uuid,
	}
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims)

	return token.SignedString([]byte(credential.JWT_SECRET))
}

var dbInstance = Db()

func IpDetails(ctx context.Context) Ip {
	p, _ := peer.FromContext(ctx)
	geoinfo := Ip{
		Ip: IptoDecimal(strings.Split(p.Addr.String(), ":")[0]),
	}
	ipRes, _ := dbInstance.Query("Select country , state , city from ip where ? between start and end", geoinfo.Ip)
	for ipRes.Next() {
		ipRes.Scan(&geoinfo.Country, &geoinfo.State, &geoinfo.City)
	}
	return geoinfo
}
func GetIP(ip string) Ip {
	geoinfo := Ip{
		Ip: IptoDecimal(ip),
	}
	ipRes, _ := dbInstance.Query("Select country , state , city from ip where ? between start and end", geoinfo.Ip)
	for ipRes.Next() {
		ipRes.Scan(&geoinfo.Country, &geoinfo.State, &geoinfo.City)
	}
	return geoinfo

}

func Tokentouser(ctx context.Context) *string {
	md, _ := metadata.FromIncomingContext(ctx)
	token, _ := jwt.Parse(md["authorization"][0], func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("err")
		}
		return []byte(credential.JWT_SECRET), nil
	})

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return Getnull(claims["uuid"].(string))
	}
	return nil
}

func Tokenstrtouser(jwtToken string) *string {

	if jwtToken == "null" || jwtToken == "" {
		return nil
	}
	token, _ := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("err")
		}
		return []byte(credential.JWT_SECRET), nil
	})

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return Getnull(claims["uuid"].(string))
	}
	return nil
}

func UnaryInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, codes.Unauthenticated.String())
	}
	values := md["authorization"]
	if len(values) > 0 {
		if !Verify(values[0]) {
			return nil, status.Error(codes.PermissionDenied, codes.PermissionDenied.String())
		}
	} else {
		return nil, status.Error(codes.PermissionDenied, codes.PermissionDenied.String())
	}
	return handler(ctx, req)
}
