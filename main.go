package main

import (
	"binchecker/api"
	"binchecker/credential"
	"binchecker/helpers"
	pb "binchecker/proto"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/plutov/paypal/v4"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/proto"
)

var countries = helpers.Byalpha2()
var db = helpers.Db()
var image = helpers.Image("image/default.jpg")
var imageSource = helpers.ImageSource("image/image.txt")
var cache = helpers.Cache()

var globalcache = helpers.GlobalRedis()
var dialect = goqu.Dialect("mysql")
var firebaseApp = helpers.FirebaseApp()
var paypalClient, _ = paypal.NewClient(credential.PAYPAL_CLIENT_ID, credential.PAYPAL_SECRET, paypal.APIBaseLive)

type server struct {
	pb.UnimplementedRPCServer
}

func main() {
	paypalClient.SetLog(os.Stdout)
	_, err := paypalClient.GetAccessToken(context.Background())
	if err != nil {
		fmt.Println(err)
	}
	go publicApi()

	lis, err := net.Listen("tcp", credential.GRPC_ADDRESS)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	//grpcServer := grpc.NewServer(grpc.StreamInterceptor(helpers.StreamInterceptor), grpc.UnaryInterceptor(helpers.UnaryInterceptor))
	tlsCredentials, _ := helpers.LoadTLSCredentials()
	grpcServer := grpc.NewServer(grpc.Creds(tlsCredentials), grpc.StreamInterceptor(helpers.StreamInterceptor), grpc.UnaryInterceptor(helpers.UnaryInterceptor))
	pb.RegisterRPCServer(grpcServer, &server{})
	log.Println("Started")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

}

func publicApi() {
	app := fiber.New()
	// enable cors for all domains
	app.Use(cors.New(
		cors.Config{
			AllowOrigins: "*",
			AllowHeaders: "Origin, Content-Type, Accept",
		},
	))
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "OK",
		})
	})

	// /login/token?locate=US
	app.Get("/login/:token", func(c *fiber.Ctx) error {
		ip := c.Get("X-Real-IP")

		locate := c.Query("locate")
		ctx := context.Background()
		received_jwtToken := c.Params("token")
		if received_jwtToken == "" {
			return c.JSON(fiber.Map{
				"message": "Token is empty",
				"status":  "NOTOK",
			})
		}

		client, err := firebaseApp.Auth(ctx)
		if err != nil {
			fmt.Println(err)
			return c.JSON(fiber.Map{
				"message": "Something went wrong while fetching user",
				"status":  "NOTOK",
			})
		}

		authToken, err := client.VerifyIDToken(ctx, received_jwtToken)

		if err != nil {
			fmt.Println(err)
			return c.JSON(fiber.Map{
				"message": "Unable to verify token",
				"status":  "NOTOK",
			})
		}

		u, err := client.GetUser(ctx, authToken.UID)

		if err != nil {
			fmt.Println(err)
			return c.JSON(fiber.Map{
				"message": "Something went wrong while fetching user",
				"status":  "NOTOK",
			})

		}

		user, err := db.Query("SELECT BIN_TO_UUID(user_id) as user_id FROM users WHERE email = ?", u.Email)
		if err != nil {
			fmt.Println(err)
			return c.JSON(fiber.Map{
				"message": "Something went wrong while fetching user",
				"status":  "NOTOK",
			})

		}
		defer user.Close()
		var user_id string

		if user.Next() {
			user.Scan(&user_id)
			token, _ := helpers.GenerateToken(user_id)
			qr, _, _ := dialect.Update("users").Set(goqu.Record{
				"image":      u.PhotoURL,
				"locate":     locate,
				"updated_at": time.Now().UnixNano() / int64(time.Millisecond),
			}).Where(goqu.C("email").Eq(u.Email)).ToSQL()
			_, err := db.Query(qr)
			if err != nil {
				fmt.Println(err)
				return c.JSON(fiber.Map{
					"message": "Something went wrong while updating user",
					"status":  "NOTOK",
				})

			}
			return c.JSON(fiber.Map{
				"message": "Logged in as " + u.Email,
				"status":  "OK",
				"token":   token,
			})

		} else {
			user_id = uuid.New().String()
			token, _ := helpers.GenerateToken(user_id)
			geoinfo := helpers.GetIP(ip)
			qr, _, _ := dialect.Insert("users").Rows(goqu.Record{
				"user_id":    goqu.L("UUID_TO_BIN(?)", helpers.Getnull(user_id)),
				"email":      helpers.Getnull(u.Email),
				"name":       helpers.Getnull(u.DisplayName),
				"image":      helpers.Getnull(u.PhotoURL),
				"country":    helpers.Getnull(geoinfo.Country),
				"state":      helpers.Getnull(geoinfo.State),
				"city":       helpers.Getnull(geoinfo.City),
				"ip":         geoinfo.Ip,
				"locate":     helpers.Getnull(locate),
				"created_at": time.Now().UnixNano() / int64(time.Millisecond),
				"updated_at": time.Now().UnixNano() / int64(time.Millisecond),
			}).ToSQL()
			_, err = db.Query(qr)
			if err != nil {
				fmt.Println(err)
				return c.JSON(fiber.Map{
					"message": "Something went wrong while creating user",
					"status":  "NOTOK",
				})

			}
			return c.JSON(fiber.Map{
				"message": "Signed up successfully",
				"status":  "OK",
				"token":   token,
			})
		}

	})

	// /lookup/:bin?token=&mode=&showTimezones&capchaKey&apiKey&token
	app.Get("/lookup/:bin", func(c *fiber.Ctx) error {
		binstr := c.Params("bin")
		mode := c.Query("mode")
		capchaKey := c.Query("capchaKey")
		apiKey := c.Query("apiKey")
		jwtToken := c.Query("token")
		user_id := helpers.Tokenstrtouser(jwtToken)
		sT := c.Query("showTimezones")
		if mode != "full" {
			mode = "minimal"
		}
		showTimezones := false
		if sT == "true" {
			showTimezones = true
		}

		// when apiKey is there use it
		// if token is there use it
		// if capchaKey is there use it
		if apiKey != "" {
			if _, err := uuid.Parse(apiKey); err != nil {
				return c.JSON(
					fiber.Map{
						"query": binstr,
						"status": fiber.Map{
							"code":    "ERROR",
							"message": "Invalid API Key",
						},
					},
				)
			} else {
				// check if enough balance and deduct money
				// 0.0009 usd per lookup
				var balance float64
				err = db.QueryRow("SELECT balance FROM users WHERE user_id = UUID_TO_BIN(?)", apiKey).Scan(&balance)
				if err != nil {
					log.Println(err)
					return c.JSON(
						fiber.Map{
							"query": binstr,
							"status": fiber.Map{
								"code":    "ERROR",
								"message": "Enable to get balance",
							},
						},
					)
				}
				if balance > credential.PRICING {
					_, err = db.Query("UPDATE users SET balance = balance-? WHERE user_id = UUID_TO_BIN(?)", credential.PRICING, apiKey)
					if err != nil {
						log.Println(err)
						return c.JSON(
							fiber.Map{
								"query": binstr,
								"status": fiber.Map{
									"code":    "ERROR",
									"message": "Something went Wrong",
								},
							},
						)
					}
				} else {
					return c.JSON(
						fiber.Map{
							"query": binstr,
							"status": fiber.Map{
								"code":    "ERROR",
								"message": "Not enough balance. Add Money and try again",
							},
						},
					)
				}
			}
		} else if capchaKey != "" {
			var captchaVerifyResponse helpers.CapchaResponse
			captchaPayloadRequest := url.Values{}
			captchaPayloadRequest.Set("secret", "secret")
			captchaPayloadRequest.Set("response", capchaKey)

			verifyCaptchaRequest, _ := http.NewRequest("POST", "https://www.google.com/recaptcha/api/siteverify", strings.NewReader(captchaPayloadRequest.Encode()))
			verifyCaptchaRequest.Header.Add("content-type", "application/x-www-form-urlencoded")
			verifyCaptchaRequest.Header.Add("cache-control", "no-cache")

			verifyCaptchaResponse, _ := http.DefaultClient.Do(verifyCaptchaRequest)
			decoder := json.NewDecoder(verifyCaptchaResponse.Body)
			decoderErr := decoder.Decode(&captchaVerifyResponse)
			defer verifyCaptchaResponse.Body.Close()
			if decoderErr != nil {
				return c.JSON(
					fiber.Map{
						"query": binstr,
						"status": fiber.Map{
							"code":    "ERROR",
							"message": "Captcha verification failed",
						},
					},
				)
			}

			if !captchaVerifyResponse.Success {
				return c.JSON(
					fiber.Map{
						"query": binstr,
						"status": fiber.Map{
							"code":    "ERROR",
							"message": "Captcha verification failed",
						},
					},
				)
			}
		} else {
			return c.JSON(
				fiber.Map{
					"query": binstr,
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "No Capcha Key Provided",
					},
				},
			)
		}
		iM := c.Query("icognitoMode")
		icognitoMode := false
		if iM == "true" {
			icognitoMode = true
		}

		ip := c.Get("X-Real-IP")
		_, err := strconv.ParseInt(binstr, 10, 64)
		if err != nil {
			return c.JSON(
				fiber.Map{
					"query": binstr,
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Invalid Bin",
					},
				},
			)

		}
		binpadstr := helpers.Pad0(binstr)
		result, err := cache.Get(binpadstr).Result()
		b := &pb.Bin{}
		var updated_at int64
		iss := &pb.Issuer{}

		var country_alpha2 string
		if err != nil {
			var row *sql.Rows
			if mode == "full" {
				row, err = db.Query("SELECT start, end, IFNULL(network,''),IFNULL(type,''),IFNULL(product_name,''),IFNULL(issuer,''),IFNULL(issuer_id,0),IFNULL(country,''), IFNULL(info,''),IFNULL(updated_at,0) FROM bins WHERE RPAD(?,11,0) BETWEEN start AND end limit 1", binstr)
			} else {
				row, err = db.Query("SELECT start, end, IFNULL(network,''),IFNULL(type,''),IFNULL(product_name,''),IFNULL(issuer,''),IFNULL(country,''), IFNULL(info,'') FROM bins WHERE RPAD(?,11,0) BETWEEN start AND end limit 1", binstr)
			}
			if err != nil {
				fmt.Println(err)
				return c.JSON(
					fiber.Map{
						"query": binstr,
						"status": fiber.Map{
							"code":    "ERROR",
							"message": "Something went wrong",
						},
					},
				)
			}
			row.Next()

			if mode == "full" {
				row.Scan(&b.Start, &b.End, &b.Network, &b.Type, &b.ProductName, &iss.Name, &iss.IssuerId, &country_alpha2, &b.Info, &updated_at)
			} else {
				row.Scan(&b.Start, &b.End, &b.Network, &b.Type, &b.ProductName, &iss.Name, &country_alpha2, &b.Info)
			}

			defer row.Close()
			if b.Start == 0 {
				return c.JSON(
					fiber.Map{
						"query": binstr,
						"status": fiber.Map{
							"code":    "NOT_FOUND",
							"message": "Bin not found",
						},
					},
				)
			} else {
				if mode == "full" {
					var card = helpers.VerifyCard(binpadstr)
					b.CvvType = card.CodeName
					b.CvvLength = card.CodeSize
					b.Length = card.Lengths

					var ctr *helpers.Country
					if iss.IssuerId > 0 {
						row, err := db.Query("SELECT name, website, email FROM issuer WHERE id = ?", &iss.IssuerId)
						if err != nil {
							fmt.Println(err)
						}
						row.Next()

						row.Scan(&iss.Name, &iss.Url, &iss.Email)
						defer row.Close()

					}
					b.Issuer = iss
					if len(country_alpha2) == 2 {
						ctr = countries[country_alpha2]
						var i = []*pb.Timezone{}
						if ctr.Timezones != nil && showTimezones {
							for _, locate := range ctr.Timezones {
								loc, err := time.LoadLocation(locate)
								if err != nil {
									fmt.Println(locate)
									fmt.Println(err)
									i = append(i, &pb.Timezone{
										IanaTimezone: locate,
										Info:         "",
									})
								} else {

									t := time.Now().In(loc)
									i = append(i, &pb.Timezone{
										IanaTimezone: locate,
										Info:         t.Format("2006-01-02 15:04:05 -0700 MST"),
									})
								}
							}
						}
						b.Country = &pb.Country{
							Name:             ctr.Name,
							Alpha2:           country_alpha2,
							Capital:          ctr.Capital,
							Emoji:            helpers.Emoji(country_alpha2),
							Region:           ctr.Region + " (" + ctr.SubRegion + ")",
							Language:         ctr.Language + " (" + ctr.LanguageAlpha2 + ")",
							CallingCode:      ctr.CallingCode,
							StartOfTheWeek:   ctr.StartofWeek,
							PostalCodeFormat: ctr.PostalCodeFormat,
							Currency: &pb.Currency{
								Name: ctr.CurrencyName + " (" + ctr.CurrencySymbol + ")",
								Code: ctr.CurrencyCode,
							},
							Locates: i,
							Numeric: int64(ctr.Numeric),
							Alpha3:  ctr.Alpha3,
						}
					}
				}
			}
		} else {
			err := proto.Unmarshal([]byte(result), b)
			if err != nil {
				fmt.Println(err)
				return c.JSON(
					fiber.Map{
						"query": binstr,
						"status": fiber.Map{
							"code":    "ERROR",
							"message": "Something went wrong",
						},
					},
				)
			}
			iss = b.Issuer
			if len(b.Country.Alpha2) == 2 && showTimezones && mode == "full" {
				country_alpha2 = b.Country.Alpha2
				ctr := countries[b.Country.Alpha2]
				var i = []*pb.Timezone{}
				if ctr.Timezones != nil {
					for _, locate := range ctr.Timezones {
						loc, err := time.LoadLocation(locate)
						if err != nil {
							fmt.Println(locate)
							fmt.Println(err)
							i = append(i, &pb.Timezone{
								IanaTimezone: locate,
								Info:         "",
							})
						} else {
							t := time.Now().In(loc)
							i = append(i, &pb.Timezone{
								IanaTimezone: locate,
								Info:         t.Format("2006-01-02 15:04:05 -0700 MST"),
							})
						}
					}
				}

				b.Country.Locates = i
			}
		}

		if b.Start > 0 && !icognitoMode {
			go func(x int64, ip_addr string, api_key string, icognito_mode bool, uid *string) {
				geoinfo := helpers.GetIP(ip_addr)
				t := time.Now().UnixNano() / int64(time.Millisecond)
				has, err := db.Query("SELECT bin FROM search_history where bin = RPAD(?,11,0) and ip = ? and searched_at >= ?", x, geoinfo.Ip, t-86400000)
				if err != nil {
					fmt.Println(err)
				}
				defer has.Close()
				if !has.Next() {
					// if apikey then deduct money
					qr, _, _ := dialect.Insert("search_history").Rows(
						goqu.Record{
							"uuid":        goqu.L("UUID_TO_BIN(?)", uuid.New().String()),
							"user_id":     goqu.L("UUID_TO_BIN(?)", uid),
							"bin":         goqu.L("RPAD(?,11,0)", x),
							"ip":          geoinfo.Ip,
							"country":     helpers.Getnull(geoinfo.Country),
							"state":       helpers.Getnull(geoinfo.State),
							"city":        helpers.Getnull(geoinfo.City),
							"action":      3,
							"searched_at": t,
						},
					).ToSQL()
					if err := globalcache.Publish("query", qr).Err(); err != nil {
						fmt.Println(qr)
						fmt.Println(err)
					}
				}
			}(b.Start, ip, apiKey, icognitoMode, user_id)
		}

		if b.Start > 0 {
			if mode == "full" {
				return c.JSON(
					fiber.Map{
						"query": binstr,
						"status": fiber.Map{
							"code":    "OK",
							"message": "Bin Found",
						},
						"data": fiber.Map{
							"start":        b.Start,
							"end":          b.End,
							"cvv_type":     b.CvvType,
							"cvv_length":   b.CvvLength,
							"length":       b.Length,
							"info":         b.Info,
							"type":         b.Type,
							"network":      b.Network,
							"product_name": b.ProductName,
							"issuer": fiber.Map{
								"name":  b.Issuer.Name,
								"url":   b.Issuer.Url,
								"email": b.Issuer.Email,
							},
							"country":      b.Country,
							"last_updated": updated_at,
						},
					},
				)
			} else {
				return c.JSON(
					fiber.Map{
						"query": binstr,
						"status": fiber.Map{
							"code":    "OK",
							"message": "Bin Found",
						},
						"data": fiber.Map{
							"start":        b.Start,
							"end":          b.End,
							"info":         b.Info,
							"type":         b.Type,
							"network":      b.Network,
							"product_name": b.ProductName,
							"issuer":       iss.Name,
							"country":      country_alpha2,
						},
					},
				)

			}

		} else {
			return c.JSON(
				fiber.Map{
					"query": binstr,
					"status": fiber.Map{
						"code":    "NOT_FOUND",
						"message": "Bin not found",
					},
				},
			)
		}

	})
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("https://binchekr.com/docs")
	})
	app.Get("/add_money", func(c *fiber.Ctx) error {
		ip := c.Get("X-Real-IP")
		jwtToken := c.Query("token")
		a := c.Query("amount")
		k, err := strconv.ParseFloat(a, 64)
		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Invalid amount",
					},
				},
			)
		}
		f := fmt.Sprintf("%.2f", k)
		amount, err := strconv.ParseFloat(f, 64)

		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Invalid amount",
					},
				},
			)
		}
		user_id := helpers.Tokenstrtouser(jwtToken)
		if user_id == nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Invalid Token",
					},
				},
			)
		}

		if amount < 1 {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Minimum amount to add is 1 USD",
					},
				},
			)
		}
		if amount > 1000 {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Maximum amount to add is 1000 USD",
					},
				},
			)
		}

		sel, err := db.Query("SELECT IFNULL(customer_id,''), IFNULL(country,''), email, name FROM users WHERE user_id = UUID_TO_BIN(?)", user_id)
		if err != nil {
			fmt.Println(err)
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Something went wrong while fetching customer id",
					},
				},
			)
		}

		defer sel.Close()
		var customer_id, country, email, name string
		for sel.Next() {
			sel.Scan(&customer_id, &country, &email, &name)
		}
		if email == "" {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "No User Found",
					},
				},
			)
		}
		// create new user
		if customer_id == "" {
			customer_id, err = api.CreateCustomer(email, name)
			if err != nil {
				fmt.Println(err)
				return c.JSON(
					fiber.Map{
						"status": fiber.Map{
							"code":    "ERROR",
							"message": err.Error(),
						},
					},
				)
			}

			_, err := db.Query("UPDATE users SET customer_id = ? where email = ?", customer_id, email)
			if err != nil {
				fmt.Println(err)
				return c.JSON(
					fiber.Map{
						"status": fiber.Map{
							"code":    "ERROR",
							"message": err.Error(),
						},
					},
				)
			}

		}
		if country == "" {
			geoinfo := helpers.GetIP(ip)
			country = geoinfo.Country
			if country == "" {
				country = "US"
			}
		}
		currency := countries[country].CurrencyCode

		// convert usd to desired currency and add fee
		price, err := api.GetPriceByCurrency(currency)
		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": err.Error(),
					},
				},
			)
		}
		amt := amount * price * 100
		amountwithoutdecimal := int64(amt)
		payment_id := uuid.New().String()
		url, err := api.GetPaymentURL(amountwithoutdecimal, customer_id, currency, payment_id)

		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": err.Error(),
					},
				},
			)
		}

		_, err = db.Query("INSERT INTO pending_transactions (payment_id, user_id, amount, created_at) VALUES (?, UUID_TO_BIN(?), ?, ?)", payment_id, user_id, amount, time.Now().Unix())

		if err != nil {
			fmt.Println(err)
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Something went wrong while saving transaction details",
					},
				},
			)
		}

		return c.JSON(
			fiber.Map{
				"status": fiber.Map{
					"code": "OK",
				},
				"url": url,
			},
		)
	})

	app.Get("/create_order", func(c *fiber.Ctx) error {
		jwtToken := c.Query("token")
		a := c.Query("amount")
		k, err := strconv.ParseFloat(a, 64)
		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Invalid amount",
					},
				},
			)
		}
		f := fmt.Sprintf("%.2f", k)
		amount, err := strconv.ParseFloat(f, 64)

		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Invalid amount",
					},
				},
			)
		}
		user_id := helpers.Tokenstrtouser(jwtToken)
		if user_id == nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Invalid Token",
					},
				},
			)
		}

		if amount < 1 {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Minimum amount to add is 1 USD",
					},
				},
			)
		}
		if amount > 1000 {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Maximum amount to add is 1000 USD",
					},
				},
			)
		}
		var email string
		err = db.QueryRow("SELECT email FROM users WHERE user_id = UUID_TO_BIN(?)", user_id).Scan(&email)
		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "No User Found",
					},
				},
			)
		}

		order, err := paypalClient.CreateOrder(context.Background(), paypal.OrderIntentCapture, []paypal.PurchaseUnitRequest{{Amount: &paypal.PurchaseUnitAmount{Value: f, Currency: "USD"}}}, &paypal.CreateOrderPayer{}, &paypal.ApplicationContext{})

		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": err.Error(),
					},
				},
			)
		}

		_, err = db.Query("INSERT INTO pending_paypal_transactions (order_id, user_id, amount, created_at) VALUES (?, UUID_TO_BIN(?), ?, ?)", order.ID, user_id, amount, time.Now().Unix())

		if err != nil {
			fmt.Println(err)
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Something went wrong while saving transaction details",
					},
				},
			)
		}

		return c.JSON(
			fiber.Map{
				"status": fiber.Map{
					"code": "OK",
				},
				"id": order.ID,
			},
		)
	})

	app.Get("/capture_order", func(c *fiber.Ctx) error {
		jwtToken := c.Query("token")
		orderId := c.Query("orderId")
		if orderId == "" {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Invalid Order ID",
					},
				},
			)
		}

		user_id := helpers.Tokenstrtouser(jwtToken)
		if user_id == nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Invalid Token",
					},
				},
			)
		}

		var email string
		err := db.QueryRow("SELECT email FROM users WHERE user_id = UUID_TO_BIN(?)", user_id).Scan(&email)
		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "No User Found",
					},
				},
			)
		}

		capture, err := paypalClient.CaptureOrder(context.Background(), orderId, paypal.CaptureOrderRequest{})

		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": err.Error(),
					},
				},
			)
		}
		if capture.Status == paypal.OrderStatusCompleted {
			var id string
			var amount float64
			err = db.QueryRow("SELECT BIN_TO_UUID(user_id), amount FROM pending_paypal_transactions WHERE order_id = ?", capture.ID).Scan(&id, &amount)
			if err != nil {
				fmt.Println(err)
				return c.JSON(
					fiber.Map{
						"status": fiber.Map{
							"code":    "ERROR",
							"message": "No Pending Transactions",
						},
					},
				)
			}
			if id != *user_id {
				return c.JSON(
					fiber.Map{
						"status": fiber.Map{
							"code":    "ERROR",
							"message": "Different Users",
						},
					},
				)
			}

			_, err = db.Query("UPDATE users set balance = balance + ? WHERE user_id = UUID_TO_BIN(?)", amount, id)
			if err != nil {
				fmt.Println(err)
				return c.JSON(
					fiber.Map{
						"status": fiber.Map{
							"code":    "ERROR",
							"message": "Something went wrong while Updating balance",
						},
					},
				)
			}
			_, err = db.Query("DELETE FROM pending_paypal_transactions WHERE order_id = ?", capture.ID)
			if err != nil {
				fmt.Println(err)
				return c.JSON(
					fiber.Map{
						"status": fiber.Map{
							"code":    "ERROR",
							"message": "Something went wrong while Updating Pending Transactions",
						},
					},
				)
			}

			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code": "OK",
					},
				},
			)
		} else {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code": capture.Status,
					},
				},
			)
		}

	})

	app.Get("/profile", func(c *fiber.Ctx) error {
		jwtToken := c.Query("token")
		apiKey := helpers.Tokenstrtouser(jwtToken)
		if apiKey == nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Invalid Token",
					},
				},
			)
		}
		var balance float64
		var user_id string
		err := db.QueryRow("SELECT balance, BIN_TO_UUID(user_id) FROM users WHERE user_id = UUID_TO_BIN(?)", apiKey).Scan(&balance, &user_id)
		if err != nil {
			return c.JSON(
				fiber.Map{
					"status": fiber.Map{
						"code":    "ERROR",
						"message": "Unable to get account",
					},
				},
			)
		}
		// var bins []int64
		// rows, err := db.Query("SELECT bin FROM search_history WHERE user_id = UUID_TO_BIN(?) ORDER BY searched_at DESC LIMIT 5", apiKey)
		// if err != nil {
		// 	return c.JSON(
		// 		fiber.Map{
		// 			"status": fiber.Map{
		// 				"code":    "ERROR",
		// 				"message": "Unable to get history",
		// 			},
		// 		},
		// 	)
		// }
		// if rows.Next() {
		// 	var bin int64
		// 	rows.Scan(&bin)
		// 	bins = append(bins, bin)
		// }
		return c.Status(200).JSON(
			fiber.Map{
				"status": fiber.Map{
					"code": "OK",
				},
				"balance": balance,
				"apiKey":  user_id,
			},
		)
	})

	// app.Post("/feedback", func(c *fiber.Ctx) error {
	// 	ip := c.Get("X-Real-IP")
	// 	go func(ipAddr string) {
	// 		payload := struct {
	// 			Feedback int   `json:"feedback"`
	// 			Bin      int64 `json:"bin"`
	// 		}{}
	// 		if err := c.BodyParser(&payload); err != nil {
	// 			fmt.Println(err)
	// 			return
	// 		}

	// 		geoinfo := helpers.GetIP(ipAddr)
	// 		t := time.Now().UnixNano() / int64(time.Millisecond)

	// 		down := 0
	// 		if payload.Feedback == 0 {
	// 			down += 1
	// 		}

	// 		qr, _, _ := dialect.Insert("feedback").Rows(
	// 			goqu.Record{
	// 				"bin":  payload.Bin,
	// 				"up":   payload.Feedback,
	// 				"down": down,
	// 			},
	// 		).OnConflict(goqu.DoUpdate(
	// 			"bin",
	// 			goqu.Record{
	// 				"up":   goqu.L("up + ?", payload.Feedback),
	// 				"down": goqu.L("down + ?", down),
	// 			},
	// 		)).ToSQL()

	// 		if err := globalcache.Publish("query", qr).Err(); err != nil {
	// 			fmt.Println(qr)
	// 			fmt.Println(err)
	// 		}

	// 		qr, _, _ = dialect.Insert("feedback_history").Rows(
	// 			goqu.Record{
	// 				"uuid":        goqu.L("UUID_TO_BIN(?)", uuid.New().String()),
	// 				"user_id":     goqu.L("UUID_TO_BIN(?)", nil),
	// 				"bin":         payload.Bin,
	// 				"ip":          geoinfo.Ip,
	// 				"feedback_at": t,
	// 			},
	// 		).ToSQL()

	// 		if err := globalcache.Publish("query", qr).Err(); err != nil {
	// 			fmt.Println(qr)
	// 			fmt.Println(err)
	// 		}

	// 	}(ip)
	// 	return c.JSON(fiber.Map{
	// 		"message": "OK",
	// 	})
	// })

	// app.Post("/improve", func(c *fiber.Ctx) error {
	// 	ip := c.Get("X-Real-IP")
	// 	payload := struct {
	// 		Bin         int64  `json:"bin"`
	// 		Network     string `json:"network"`
	// 		Type        string `json:"type"`
	// 		ProductName string `json:"product_name"`
	// 		Issuer      string `json:"issuer"`
	// 		Country     string `json:"country"`
	// 		Text        string `json:"text"`
	// 	}{}
	// 	go func(ipAddr string) {
	// 		t := time.Now().UnixNano() / int64(time.Millisecond)
	// 		geoinfo := helpers.GetIP(ipAddr)
	// 		qr, _, _ := dialect.Insert("report").
	// 			Rows(
	// 				goqu.Record{
	// 					"uuid":         goqu.L("UUID_TO_BIN(?)", uuid.New().String()),
	// 					"user_id":      goqu.L("UUID_TO_BIN(?)", nil),
	// 					"bin":          payload.Bin,
	// 					"ip":           geoinfo.Ip,
	// 					"network":      helpers.Getnull(payload.Network),
	// 					"type":         helpers.Getnull(payload.Type),
	// 					"product_name": helpers.Getnull(payload.ProductName),
	// 					"issuer":       helpers.Getnull(payload.Issuer),
	// 					"country":      helpers.Getnull(payload.Country),
	// 					"text":         helpers.Getnull(payload.Text),
	// 					"reported_at":  t,
	// 				},
	// 			).ToSQL()

	// 		if err := globalcache.Publish("update", qr).Err(); err != nil {
	// 			fmt.Println(qr)
	// 			fmt.Println(err)
	// 		}

	// 	}(ip)
	// 	return c.JSON(fiber.Map{
	// 		"message": "OK",
	// 	})
	// })

	// other routes return 404
	app.Use(func(c *fiber.Ctx) error {
		return c.Redirect("/")
	})
	app.Listen(":8080")

}

func (s *server) BulkLookup(in *pb.BulkRequest, stream pb.RPC_BulkLookupServer) error {
	binarr := []int64{}
	for _, bin := range in.Bin {
		binstr := strconv.FormatInt(bin, 10)
		binpadstr := helpers.Pad0(binstr)
		result, err := cache.Get(binpadstr).Result()
		if err != nil {
			selBins, err := db.Query("SELECT start, end, IFNULL(network,''),IFNULL(type,''),IFNULL(product_name,''),IFNULL(issuer,''),IFNULL(country,'') FROM bins WHERE RPAD(?,11,0) BETWEEN start AND end", bin)
			if err != nil {
				fmt.Println(err)
			}
			defer selBins.Close()
			b := &pb.Bin{}
			if selBins.Next() {
				iss := &pb.Issuer{}
				c := &pb.Country{}
				selBins.Scan(&b.Start, &b.End, &b.Network, &b.Type, &b.ProductName, &iss.Name, &c.Alpha2)
				b.Issuer = iss

				if len(c.Alpha2) >= 2 {
					c.Emoji = helpers.Emoji(c.Alpha2)
					b.Country = c
				} else {
					b.Country = c
				}
				stream.Send(b)
				binarr = append(binarr, b.Start)
			}
		} else {
			b := &pb.Bin{}
			proto.Unmarshal([]byte(result), b)
			stream.Send(b)
		}
	}

	if len(binarr) > 0 && !in.Incognito {
		go func(bins []int64, context context.Context) {
			user_id := helpers.Tokentouser(context)
			t := time.Now().UnixNano() / int64(time.Millisecond)
			geoinfo := helpers.IpDetails(context)
			var qr string
			if user_id != nil {
				qr, _, _ = dialect.From("search_history").Where(goqu.Ex{"bin": bins, "user_id": user_id, "searched_at": goqu.Op{"gte": t - 86400000}}).ToSQL()
			} else {
				qr, _, _ = dialect.From("search_history").Where(goqu.Ex{"bin": bins, "ip": geoinfo.Ip, "searched_at": goqu.Op{"gte": t - 86400000}}).ToSQL()
			}
			has, err := db.Query(qr)
			if err != nil {
				fmt.Println(err)
			}
			defer has.Close()
			for has.Next() {
				var bin int64
				has.Scan(&bin)
				bins = helpers.Remove(bins, bin)
			}
			if len(bins) > 0 {
				insert := dialect.Insert("search_history")
				for _, bin := range bins {
					insert.Rows(
						goqu.Record{
							"uuid":        goqu.L("UUID_TO_BIN(?)", uuid.New().String()),
							"user_id":     goqu.L("UUID_TO_BIN(?)", user_id),
							"bin":         bin,
							"ip":          geoinfo.Ip,
							"country":     helpers.Getnull(geoinfo.Country),
							"state":       helpers.Getnull(geoinfo.State),
							"city":        helpers.Getnull(geoinfo.City),
							"action":      0,
							"searched_at": t,
						},
					)
				}
				qr, _, _ := insert.ToSQL()
				if err := globalcache.Publish("query", qr).Err(); err != nil {
					fmt.Println(qr)
					fmt.Println(err)
				}
			}

		}(binarr, stream.Context())
	}
	return nil
}

func (s *server) DeleteHistory(ctx context.Context, in *pb.HistoryRequest) (*pb.Message, error) {
	user_id := helpers.Tokentouser(ctx)
	if user_id == nil {
		return &pb.Message{
			Message: "Invalid Login",
			Status:  false,
		}, nil
	}
	go func(uid *string) {
		if user_id != nil {
			qr, _, _ := dialect.Update("search_history").Set(
				goqu.Record{
					"user_id": nil,
				}).Where(goqu.C("user_id").Eq(goqu.L("UUID_TO_BIN(?)", uid))).ToSQL()

			if err := globalcache.Publish("query", qr).Err(); err != nil {
				fmt.Println(qr)
				fmt.Println(err)
			}
		}
	}(user_id)
	return &pb.Message{
		Status:  true,
		Message: "OK",
	}, nil
}

func (s *server) Stat(ctx context.Context, in *pb.StatReq) (*pb.StatResponse, error) {
	var data []*pb.StatMap
	selectData, _ := db.Query("SELECT IFNULL(day_bin,0), IFNULL(week_bin,0), IFNULL(month_bin,0)  FROM global_stat")
	defer selectData.Close()
	var day []int64
	var week []int64
	var month []int64
	for selectData.Next() {
		var daybin int64
		var weekbin int64
		var monthbin int64
		selectData.Scan(&daybin, &weekbin, &monthbin)
		if daybin > 0 {
			day = append(day, daybin)
		} else if weekbin > 0 {
			week = append(week, weekbin)
		} else {
			month = append(month, monthbin)
		}

	}

	data = append(data, &pb.StatMap{
		Name: "Top Bins Searched in Last 24 Hours",
		Data: day,
	})
	data = append(data, &pb.StatMap{
		Name: "Top Bins Searched in Last 7 Days",
		Data: week,
	})
	data = append(data, &pb.StatMap{
		Name: "Top Bins Searched in Last 30 Days",
		Data: month,
	})

	return &pb.StatResponse{
		Data: data,
	}, nil
}

func (s *server) AddBin(ctx context.Context, in *pb.ReportRequest) (*pb.Message, error) {
	go func(data *pb.ReportRequest, context context.Context) {
		t := time.Now().UnixNano() / int64(time.Millisecond)
		geoinfo := helpers.IpDetails(context)
		user_id := helpers.Tokentouser(context)
		qr, _, _ := dialect.Insert("report").
			Rows(
				goqu.Record{
					"uuid":         goqu.L("UUID_TO_BIN(?)", uuid.New().String()),
					"user_id":      goqu.L("UUID_TO_BIN(?)", user_id),
					"bin":          goqu.L("RPAD(? ,11,'0'", data.Bin),
					"ip":           geoinfo.Ip,
					"network":      helpers.Getnull(data.Network),
					"type":         helpers.Getnull(data.Type),
					"product_name": helpers.Getnull(data.ProductName),
					"issuer":       helpers.Getnull(data.Issuer),
					"country":      helpers.Getnull(data.Country),
					"text":         "Add Bin: " + data.Text,
					"reported_at":  t,
				},
			).ToSQL()

		if err := globalcache.Publish("update", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
		}

	}(in, ctx)
	return &pb.Message{
		Status:  true,
		Message: "OK",
	}, nil
}

func (s *server) Get_Image(ctx context.Context, in *pb.ImageRequest) (*pb.Image, error) {
	return &pb.Image{
		Image: image,
	}, nil
}

func (s *server) SetFav(context context.Context, in *pb.Fav) (*pb.Message, error) {
	user_id := helpers.Tokentouser(context)
	if user_id == nil {
		return &pb.Message{
			Message: "Invalid Login",
			Status:  false,
		}, nil
	}
	switch in.What {
	case pb.Fav_ADD:
		qr, _, _ := dialect.Update("users").Set(goqu.Record{"bins": goqu.L("IF(JSON_CONTAINS(bins, '\"?\"'), bins, JSON_ARRAY_APPEND(bins, '$', '?'))", in.Bin, in.Bin),
			"updated_at": time.Now().UnixNano() / int64(time.Millisecond),
		}).Where(goqu.Ex{
			"user_id": goqu.L("UUID_TO_BIN(?)", user_id),
		}).ToSQL()
		if err := globalcache.Publish("query", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
			return &pb.Message{Status: false}, nil
		} else {
			return &pb.Message{Status: true}, nil
		}

	case pb.Fav_DELETE:
		qr, _, _ := dialect.Update("users").Set(
			goqu.Record{"bins": goqu.L("IFNULL(JSON_REMOVE(bins, JSON_UNQUOTE(JSON_SEARCH(bins, 'one','?'))),bins)", in.Bin),
				"updated_at": time.Now().UnixNano() / int64(time.Millisecond),
			}).Where(goqu.Ex{
			"user_id": goqu.L("UUID_TO_BIN(?)", user_id),
		}).ToSQL()
		if err := globalcache.Publish("query", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
			return &pb.Message{Status: false}, nil
		} else {
			return &pb.Message{Status: true}, nil
		}
	}
	return &pb.Message{
		Message: "ok",
		Status:  true,
	}, nil
}

func (s *server) GetAccount(ctx context.Context, in *pb.AccountRequest) (*pb.AccountResponse, error) {
	user_id := helpers.Tokentouser(ctx)

	if user_id != nil {
		row, err := db.Query("SELECT bins FROM users WHERE user_id = UUID_TO_BIN(?)", user_id)
		if err != nil {
			fmt.Println(err)
		}
		defer row.Close()
		var bins json.RawMessage
		var binsint []int64
		if row.Next() {
			if err := row.Scan(&bins); err != nil {
				fmt.Println(err)
			}
			var binsstr []string
			json.Unmarshal(bins, &binsstr)

			for _, bin := range binsstr {
				binint, err := strconv.ParseInt(bin, 10, 64)
				if err != nil {
					fmt.Println(err)

				}
				binsint = append(binsint, binint)
			}

		}

		row, err = db.Query("SELECT bin_to_uuid(uuid), name , bins FROM savedlist WHERE user_id = UUID_TO_BIN(?)", user_id)
		if err != nil {
			fmt.Println(err)
		}
		defer row.Close()
		var list []*pb.Save
		for row.Next() {
			var uuid, name string
			var bins json.RawMessage
			if err := row.Scan(&uuid, &name, &bins); err != nil {
				fmt.Println(err)
			}
			var binsstr []string
			json.Unmarshal(bins, &binsstr)

			var binsint []int64
			for _, bin := range binsstr {
				binint, err := strconv.ParseInt(bin, 10, 64)
				if err != nil {
					fmt.Println(err)

				}
				binsint = append(binsint, binint)
			}
			list = append(list, &pb.Save{
				Uuid: uuid,
				Name: name,
				Bins: binsint,
			})
		}

		selectQuery, _ := db.Query("SELECT bin,searched_at FROM search_history WHERE user_id = UUID_TO_BIN(?) ORDER BY searched_at DESC", user_id)
		defer selectQuery.Close()
		var data []*pb.Stat
		for selectQuery.Next() {
			d := &pb.Stat{}
			selectQuery.Scan(&d.Bin, &d.At)
			data = append(data, d)
		}

		return &pb.AccountResponse{
			Save:    list,
			Fav:     binsint,
			History: data,
		}, nil
	} else {
		return &pb.AccountResponse{}, nil
	}
}

func (s *server) Set_List(context context.Context, in *pb.Save) (*pb.Message, error) {
	user_id := helpers.Tokentouser(context)
	if user_id == nil {
		return &pb.Message{
			Message: "Invalid Login",
			Status:  false,
		}, nil
	}
	switch in.GetWhat() {
	case pb.Save_ADD_LIST:
		qr, _, _ := dialect.Insert("savedlist").Rows(
			goqu.Record{"uuid": goqu.L("UUID_TO_BIN(?)", in.Uuid),
				"user_id":    goqu.L("UUID_TO_BIN(?)", user_id),
				"name":       in.Name,
				"created_at": time.Now().UnixNano() / int64(time.Millisecond),
			},
		).ToSQL()
		if err := globalcache.Publish("query", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
			return &pb.Message{Status: false}, nil
		} else {
			return &pb.Message{Status: true}, nil
		}
	case pb.Save_DELETE_LIST:
		qr, _, _ := dialect.Delete("savedlist").Where(goqu.Ex{
			"user_id": goqu.L("UUID_TO_BIN(?)", user_id),
			"uuid":    goqu.L("UUID_TO_BIN(?)", in.Uuid),
		}).ToSQL()

		if err := globalcache.Publish("query", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
			return &pb.Message{Status: false}, nil
		} else {
			return &pb.Message{Status: true}, nil
		}
	case pb.Save_ADD_BIN_T0_LIST:
		qr, _, _ := dialect.Update("savedlist").Set(goqu.Record{"bins": goqu.L("IF(JSON_CONTAINS(bins, '\"?\"'), bins, JSON_ARRAY_APPEND(bins, '$', '?'))", in.Bins[0], in.Bins[0]),
			"updated_at": time.Now().UnixNano() / int64(time.Millisecond),
		}).Where(goqu.Ex{
			"user_id": goqu.L("UUID_TO_BIN(?)", user_id),
			"uuid":    goqu.L("UUID_TO_BIN(?)", in.Uuid),
		}).ToSQL()

		if err := globalcache.Publish("query", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
			return &pb.Message{Status: false}, nil
		} else {
			return &pb.Message{Status: true}, nil
		}

	case pb.Save_DELETE_BIN_FROM_LIST:
		qr, _, _ := dialect.Update("savedlist").Set(
			goqu.Record{"bins": goqu.L("IFNULL(JSON_REMOVE(bins, JSON_UNQUOTE(JSON_SEARCH(bins, 'one','?'))),bins)", in.Bins[0]),
				"updated_at": time.Now().UnixNano() / int64(time.Millisecond),
			}).Where(goqu.Ex{
			"user_id": goqu.L("UUID_TO_BIN(?)", user_id),
			"uuid":    goqu.L("UUID_TO_BIN(?)", in.Uuid),
		}).ToSQL()

		if err := globalcache.Publish("query", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
			return &pb.Message{Status: false}, nil
		} else {
			return &pb.Message{Status: true}, nil
		}

	case pb.Save_RENAME_LIST:
		qr, _, _ := dialect.Update("savedlist").Set(goqu.Record{"name": in.Name,
			"updated_at": time.Now().UnixNano() / int64(time.Millisecond),
		}).Where(goqu.Ex{
			"user_id": goqu.L("UUID_TO_BIN(?)", user_id),
			"uuid":    goqu.L("UUID_TO_BIN(?)", in.Uuid),
		}).ToSQL()
		if err := globalcache.Publish("query", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
			return &pb.Message{Status: false}, nil
		} else {
			return &pb.Message{Status: true}, nil
		}
	}
	return &pb.Message{Status: false}, nil
}
func (s *server) Get_List(context context.Context, in *pb.Save) (*pb.SaveList, error) {
	user_id := helpers.Tokentouser(context)
	if user_id == nil {
		return &pb.SaveList{}, nil
	}
	row, err := db.Query("SELECT bin_to_uuid(uuid), name , bins FROM savedlist WHERE user_id = UUID_TO_BIN(?)", user_id)
	if err != nil {
		fmt.Println(err)
	}
	defer row.Close()
	var list []*pb.Save
	for row.Next() {
		var uuid, name string
		var bins json.RawMessage
		if err := row.Scan(&uuid, &name, &bins); err != nil {
			fmt.Println(err)
		}
		var binsstr []string
		json.Unmarshal(bins, &binsstr)

		var binsint []int64
		for _, bin := range binsstr {
			binint, err := strconv.ParseInt(bin, 10, 64)
			if err != nil {
				fmt.Println(err)

			}
			binsint = append(binsint, binint)
		}
		list = append(list, &pb.Save{
			Uuid: uuid,
			Name: name,
			Bins: binsint,
		})
	}

	return &pb.SaveList{Save: list}, nil
}

func (s *server) Find_Bin(ctx context.Context, in *pb.SearchRequest) (*pb.BinList, error) {
	binstr := strconv.FormatInt(in.Bin, 10)
	binlist := []*pb.Bin{}
	var err error
	var selBins *sql.Rows
	page := uint(in.Page * 50)

	if len(binstr) > 6 {
		selBins, err = db.Query("SELECT start, end, IFNULL(network,''),IFNULL(type,''),IFNULL(product_name,''),IFNULL(issuer,''),IFNULL(country,'') FROM bins WHERE RPAD(?,11,0) BETWEEN start AND end limit ?, 50 ", in.Bin, page)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		var network = in.GetNetwork()
		var product = in.GetProductName()
		var issuer = in.GetIssuer()
		var country = in.GetCountry()
		var Type = in.GetType()

		ex := []goqu.Expression{}
		if network == "NULL" {
			ex = append(ex, goqu.Ex{"network": nil})
		} else if network != "" {
			ex = append(ex, goqu.Ex{"network": network})
		}

		if product == "NULL" {
			ex = append(ex, goqu.Ex{"product_name": nil})
		} else if product != "" {
			ex = append(ex, goqu.Ex{"product_name": product})
		}

		if issuer == "NULL" {
			ex = append(ex, goqu.Ex{"issuer": nil})
		} else if issuer != "" {
			ex = append(ex, goqu.L("issuer LIKE ?", issuer+"%"))
		}

		if country == "NULL" {
			ex = append(ex, goqu.Ex{"country": nil})
		} else if country != "" {
			ex = append(ex, goqu.Ex{"country": country})
		}

		if Type == "NULL" {
			ex = append(ex, goqu.Ex{"type": nil})
		} else if Type != "" {
			ex = append(ex, goqu.Ex{"type": Type})
		}

		if in.GetBin() > 0 {
			binstr := strconv.FormatInt(in.Bin, 10)
			start := helpers.Pad0(binstr)
			end := helpers.Pad9(binstr)
			ex = append(ex, goqu.Ex{"start": goqu.Op{"between": goqu.Range(start, end)}})
			sel := dialect.From("bins").Limit(50).Offset(page).Select("start", "end", goqu.L("IFNULL(network,'')"), goqu.L("IFNULL(type,'')"), goqu.L("IFNULL(product_name,'')"), goqu.L("IFNULL(issuer,'')"), goqu.L("IFNULL(country,'')")).Where(ex...)
			qr, _, _ := sel.ToSQL()

			selBins, err = db.Query(qr)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			sel := dialect.From("bins").Limit(50).Offset(page).Select("start", "end", goqu.L("IFNULL(network,'')"), goqu.L("IFNULL(type,'')"), goqu.L("IFNULL(product_name,'')"), goqu.L("IFNULL(issuer,'')"), goqu.L("IFNULL(country,'')")).Where(ex...)
			qr, _, _ := sel.ToSQL()
			selBins, err = db.Query(qr)
			if err != nil {
				fmt.Println(err)
			}
		}

	}

	defer selBins.Close()

	for selBins.Next() {
		b := &pb.Bin{}
		iss := &pb.Issuer{}
		c := &pb.Country{}
		selBins.Scan(&b.Start, &b.End, &b.Network, &b.Type, &b.ProductName, &iss.Name, &c.Alpha2)
		b.Issuer = iss

		if len(c.Alpha2) >= 2 {
			c.Emoji = helpers.Emoji(c.Alpha2)
			b.Country = c
		} else {
			b.Country = c
		}
		binlist = append(binlist, b)
	}

	return &pb.BinList{
		Binlist:     binlist,
		CurrentPage: in.Page,
	}, nil
}
func (s *server) Lookup(ctx context.Context, in *pb.BinRequest) (*pb.Bin, error) {
	binstr := strconv.FormatInt(in.Bin, 10)
	binpadstr := helpers.Pad0(binstr)
	result, err := cache.Get(binpadstr).Result()
	b := &pb.Bin{}
	if err != nil {
		iss := &pb.Issuer{}
		var c string
		row, err := db.Query("SELECT start, end, IFNULL(network,''),IFNULL(type,''),IFNULL(product_name,''),IFNULL(issuer,''),IFNULL(issuer_id,0),IFNULL(country,''), IFNULL(info,''),IFNULL(updated_at,0) FROM bins WHERE start = RPAD(?,11,0)", in.Bin)
		if err != nil {
			fmt.Println(err)
		}

		row.Next()
		row.Scan(&b.Start, &b.End, &b.Network, &b.Type, &b.ProductName, &iss.Name, &iss.IssuerId, &c, &b.Info, &b.LastUpdated)
		defer row.Close()

		if b.Start > 0 {
			b.Issuer = iss
			var card = helpers.VerifyCard(strconv.FormatInt(b.Start, 10))
			b.CvvType = card.CodeName
			b.CvvLength = card.CodeSize
			b.Length = card.Lengths

			var ctr = countries[c]
			if len(c) == 2 {
				var i = []*pb.Timezone{}
				if in.Timezone {
					if ctr.Timezones != nil {
						for _, locate := range ctr.Timezones {
							loc, err := time.LoadLocation(locate)
							if err != nil {
								fmt.Println(locate)
								fmt.Println(err)
								i = append(i, &pb.Timezone{
									IanaTimezone: locate,
									Info:         "",
								})
							} else {
								t := time.Now().In(loc)
								i = append(i, &pb.Timezone{
									IanaTimezone: locate,
									Info:         t.Format("2006-01-02 15:04:05 -0700 MST"),
								})
							}
						}
					}
				}
				// if iss.IssuerId > 0 {
				// 	row, err := db.Query("SELECT name, website, email FROM issuer WHERE id = ?", iss.IssuerId)
				// 	if err != nil {
				// 		fmt.Println(err)
				// 	}
				// 	row.Next()
				// 	row.Scan(&iss.Name, &iss.Url, &iss.Email)
				// 	defer row.Close()
				// }
				b.Country = &pb.Country{
					Name:             ctr.Name,
					Alpha2:           c,
					Capital:          ctr.Capital,
					Emoji:            helpers.Emoji(c),
					Region:           ctr.Region + " (" + ctr.SubRegion + ")",
					Language:         ctr.Language + " (" + ctr.LanguageAlpha2 + ")",
					CallingCode:      ctr.CallingCode,
					StartOfTheWeek:   ctr.StartofWeek,
					PostalCodeFormat: ctr.PostalCodeFormat,
					Currency: &pb.Currency{
						Name: ctr.CurrencyName + " (" + ctr.CurrencySymbol + ")",
						Code: ctr.CurrencyCode,
					},
					Locates: i,
					Numeric: int64(ctr.Numeric),
					Alpha3:  ctr.Alpha3,
				}
			}

		}

		go func(str string, bsr *pb.Bin) {
			var c, err = proto.Marshal(bsr)
			if err != nil {
				fmt.Println(err)
				return
			}
			bsr.Country.Locates = nil
			globalcache.Set(str, c, 24*time.Hour)
		}(binpadstr, b)

	} else {
		proto.Unmarshal([]byte(result), b)
		if len(b.Country.Alpha2) == 2 {
			ctr := countries[b.Country.Alpha2]
			var i = []*pb.Timezone{}
			if in.Timezone {
				if ctr.Timezones != nil {
					for _, locate := range ctr.Timezones {
						loc, err := time.LoadLocation(locate)
						if err != nil {
							fmt.Println(locate)
							fmt.Println(err)
							i = append(i, &pb.Timezone{
								IanaTimezone: locate,
								Info:         "",
							})
						} else {
							t := time.Now().In(loc)
							i = append(i, &pb.Timezone{
								IanaTimezone: locate,
								Info:         t.Format("2006-01-02 15:04:05 -0700 MST"),
							})
						}
					}
				}

				b.Country.Locates = i
			}

		}
	}

	if b.Start > 0 && !in.Incognito {
		go func(x int64, context context.Context) {
			user_id := helpers.Tokentouser(context)
			t := time.Now().UnixNano() / int64(time.Millisecond)
			var has *sql.Rows
			var err error
			geoinfo := helpers.IpDetails(context)
			if user_id != nil {
				has, err = db.Query("SELECT bin FROM search_history where bin = RPAD(?,11,0) and user_id = UUID_TO_BIN(?) and searched_at >= ?", x, user_id, t-86400000)
			} else {
				has, err = db.Query("SELECT bin FROM search_history where bin = RPAD(?,11,0) and ip = ? and searched_at >= ?", x, geoinfo.Ip, t-86400000)
			}
			if err != nil {
				fmt.Println(err)
			}

			defer has.Close()
			if !has.Next() {

				qr, _, _ := dialect.Insert("search_history").Rows(
					goqu.Record{
						"uuid":        goqu.L("UUID_TO_BIN(?)", uuid.New().String()),
						"user_id":     goqu.L("UUID_TO_BIN(?)", user_id),
						"bin":         goqu.L("RPAD(?,11,0)", x),
						"ip":          geoinfo.Ip,
						"country":     helpers.Getnull(geoinfo.Country),
						"state":       helpers.Getnull(geoinfo.State),
						"city":        helpers.Getnull(geoinfo.City),
						"action":      0,
						"searched_at": t,
					},
				).ToSQL()
				if err := globalcache.Publish("query", qr).Err(); err != nil {
					fmt.Println(qr)
					fmt.Println(err)
				}
			}
		}(b.Start, ctx)
	}
	return b, nil
}
func (s *server) Find_6DigitBin(ctx context.Context, in *pb.Search6DigitRequest) (*pb.BinList6Digit, error) {
	binstr := strconv.FormatInt(in.Bin, 10)
	binlist := []*pb.Bin6Digit{}
	var err error
	var selBins *sql.Rows
	page := uint(in.Page * 50)

	if len(binstr) == 6 {
		selBins, err = db.Query("SELECT bin, IFNULL(network,''),IFNULL(type,''),IFNULL(product,''),IFNULL(issuer,''),IFNULL(country,'') FROM oldbins WHERE bin = ? limit ?, 50", in.Bin, page)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		var network = in.GetNetwork()
		var product = in.GetProductName()
		var issuer = in.GetIssuer()
		var country = in.GetCountry()
		var Type = in.GetType()

		ex := []goqu.Expression{}
		if network == "NULL" {
			ex = append(ex, goqu.Ex{"network": nil})
		} else if network != "" {
			ex = append(ex, goqu.Ex{"network": network})
		}

		if product == "NULL" {
			ex = append(ex, goqu.Ex{"product": nil})
		} else if product != "" {
			ex = append(ex, goqu.Ex{"product": product})
		}

		if issuer == "NULL" {
			ex = append(ex, goqu.Ex{"issuer": nil})
		} else if issuer != "" {
			ex = append(ex, goqu.L("issuer LIKE ?", issuer+"%"))
		}

		if country == "NULL" {
			ex = append(ex, goqu.Ex{"country": nil})
		} else if country != "" {
			ex = append(ex, goqu.Ex{"country": country})
		}

		if Type == "NULL" {
			ex = append(ex, goqu.Ex{"type": nil})
		} else if Type != "" {
			ex = append(ex, goqu.Ex{"type": Type})
		}

		if in.GetBin() > 0 {
			//binstr := strconv.FormatInt(in.Bin, 10)
			// start := helpers.Pad0(binstr)
			// end := helpers.Pad9(binstr)
			//ex = append(ex, goqu.Ex{"start": goqu.Op{"between": goqu.Range(start, end)}})
			// bin like 123456%
			ex = append(ex, goqu.L("bin LIKE ?", binstr+"%"))
			sel := dialect.From("oldbins").Limit(50).Offset(page).Select("bin", goqu.L("IFNULL(network,'')"), goqu.L("IFNULL(type,'')"), goqu.L("IFNULL(product,'')"), goqu.L("IFNULL(issuer,'')"), goqu.L("IFNULL(country,'')")).Where(ex...)
			qr, _, _ := sel.ToSQL()

			selBins, err = db.Query(qr)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			sel := dialect.From("oldbins").Limit(50).Offset(page).Select("bin", goqu.L("IFNULL(network,'')"), goqu.L("IFNULL(type,'')"), goqu.L("IFNULL(product,'')"), goqu.L("IFNULL(issuer,'')"), goqu.L("IFNULL(country,'')")).Where(ex...)
			qr, _, _ := sel.ToSQL()
			selBins, err = db.Query(qr)
			if err != nil {
				fmt.Println(err)
			}
		}

	}

	defer selBins.Close()

	for selBins.Next() {
		b := &pb.Bin6Digit{}

		c := &pb.Country{}
		selBins.Scan(&b.Bin, &b.Network, &b.Type, &b.ProductName, &b.Issuer, &c.Alpha2)

		if len(c.Alpha2) >= 2 {
			c.Emoji = helpers.Emoji(c.Alpha2)
			b.Country = c
		} else {
			b.Country = c
		}
		binlist = append(binlist, b)
	}

	return &pb.BinList6Digit{
		Binlist:     binlist,
		CurrentPage: in.Page,
	}, nil
}
func (s *server) Lookup_6DigitBin(ctx context.Context, in *pb.Bin6DigitRequest) (*pb.Bin6Digit, error) {
	b := &pb.Bin6Digit{}
	var c string
	row, err := db.Query("SELECT bin, IFNULL(network,''),IFNULL(type,''),IFNULL(product,''),IFNULL(issuer,''),IFNULL(country,'') FROM oldbins WHERE bin = ?", in.Bin)
	if err != nil {
		fmt.Println(err)
	}

	row.Next()
	row.Scan(&b.Bin, &b.Network, &b.Type, &b.ProductName, &b.Issuer, &c)
	defer row.Close()
	if b.Bin > 0 {
		// var card = helpers.VerifyCard(strconv.FormatInt(b.Bin, 10))
		// b.CvvType = card.CodeName
		// b.CvvLength = card.CodeSize
		// b.Length = card.Lengths

		var ctr = countries[c]
		if len(c) == 2 {
			var i = []*pb.Timezone{}
			if in.Timezone {
				if ctr.Timezones != nil {
					for _, locate := range ctr.Timezones {
						loc, err := time.LoadLocation(locate)
						if err != nil {
							fmt.Println(locate)
							fmt.Println(err)
							i = append(i, &pb.Timezone{
								IanaTimezone: locate,
								Info:         "",
							})
						} else {
							t := time.Now().In(loc)
							i = append(i, &pb.Timezone{
								IanaTimezone: locate,
								Info:         t.Format("2006-01-02 15:04:05 -0700 MST"),
							})
						}
					}
				}
			}

			b.Country = &pb.Country{
				Name:             ctr.Name,
				Alpha2:           c,
				Capital:          ctr.Capital,
				Emoji:            helpers.Emoji(c),
				Region:           ctr.Region + " (" + ctr.SubRegion + ")",
				Language:         ctr.Language + " (" + ctr.LanguageAlpha2 + ")",
				CallingCode:      ctr.CallingCode,
				StartOfTheWeek:   ctr.StartofWeek,
				PostalCodeFormat: ctr.PostalCodeFormat,
				Currency: &pb.Currency{
					Name: ctr.CurrencyName + " (" + ctr.CurrencySymbol + ")",
					Code: ctr.CurrencyCode,
				},
				Locates: i,
				Numeric: int64(ctr.Numeric),
				Alpha3:  ctr.Alpha3,
			}
		}

	}

	if b.Bin > 0 && !in.Incognito {
		go func(x int64, context context.Context) {
			user_id := helpers.Tokentouser(context)
			t := time.Now().UnixNano() / int64(time.Millisecond)
			var has *sql.Rows
			var err error
			geoinfo := helpers.IpDetails(context)

			if user_id != nil {
				has, err = db.Query("SELECT bin FROM search_history where bin = RPAD(?,11,0) and user_id = UUID_TO_BIN(?) and searched_at >= ?", x, user_id, t-86400000)
			} else {
				has, err = db.Query("SELECT bin FROM search_history where bin = RPAD(?,11,0) and ip = ? and searched_at >= ?", x, geoinfo.Ip, t-86400000)
			}
			if err != nil {
				fmt.Println(err)
			}

			defer has.Close()
			if !has.Next() {
				qr, _, _ := dialect.Insert("search_history").Rows(
					goqu.Record{
						"uuid":        goqu.L("UUID_TO_BIN(?)", uuid.New().String()),
						"user_id":     goqu.L("UUID_TO_BIN(?)", user_id),
						"bin":         goqu.L("RPAD(?,11,0)", x),
						"ip":          geoinfo.Ip,
						"country":     helpers.Getnull(geoinfo.Country),
						"state":       helpers.Getnull(geoinfo.State),
						"city":        helpers.Getnull(geoinfo.City),
						"action":      0,
						"searched_at": t,
					},
				).ToSQL()
				if err := globalcache.Publish("query", qr).Err(); err != nil {
					fmt.Println(qr)
					fmt.Println(err)
				}
			}
		}(b.Bin, ctx)
	}
	return b, nil
}

func (s *server) Report(ctx context.Context, in *pb.ReportRequest) (*pb.Message, error) {
	go func(data *pb.ReportRequest, context context.Context) {
		t := time.Now().UnixNano() / int64(time.Millisecond)
		geoinfo := helpers.IpDetails(context)

		user_id := helpers.Tokentouser(context)
		qr, _, _ := dialect.Insert("report").
			Rows(
				goqu.Record{
					"uuid":         goqu.L("UUID_TO_BIN(?)", uuid.New().String()),
					"user_id":      goqu.L("UUID_TO_BIN(?)", user_id),
					"bin":          data.Bin,
					"ip":           geoinfo.Ip,
					"network":      helpers.Getnull(data.Network),
					"type":         helpers.Getnull(data.Type),
					"product_name": helpers.Getnull(data.ProductName),
					"issuer":       helpers.Getnull(data.Issuer),
					"country":      helpers.Getnull(data.Country),
					"text":         helpers.Getnull(data.Text),
					"reported_at":  t,
				},
			).ToSQL()

		if err := globalcache.Publish("update", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
		}

	}(in, ctx)
	return &pb.Message{
		Status:  true,
		Message: "OK",
	}, nil
}

func (s *server) Send_Feedback(ctx context.Context, in *pb.FeedbackRequest) (*pb.Message, error) {
	go func(data *pb.FeedbackRequest, context context.Context) {
		geoinfo := helpers.IpDetails(context)
		t := time.Now().UnixNano() / int64(time.Millisecond)
		user_id := helpers.Tokentouser(context)

		down := 0
		if in.Feedback == 0 {
			down += 1
		}

		qr, _, _ := dialect.Insert("feedback").Rows(
			goqu.Record{
				"bin":  in.Bin,
				"up":   in.Feedback,
				"down": down,
			},
		).OnConflict(goqu.DoUpdate(
			"bin",
			goqu.Record{
				"up":   goqu.L("up + ?", in.Feedback),
				"down": goqu.L("down + ?", down),
			},
		)).ToSQL()

		if err := globalcache.Publish("query", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
		}

		qr, _, _ = dialect.Insert("feedback_history").Rows(
			goqu.Record{
				"uuid":        goqu.L("UUID_TO_BIN(?)", uuid.New().String()),
				"user_id":     goqu.L("UUID_TO_BIN(?)", user_id),
				"bin":         data.Bin,
				"ip":          geoinfo.Ip,
				"feedback_at": t,
			},
		).ToSQL()

		if err := globalcache.Publish("query", qr).Err(); err != nil {
			fmt.Println(qr)
			fmt.Println(err)
		}

	}(in, ctx)
	return &pb.Message{
		Status:  true,
		Message: "OK",
	}, nil
}

func (s *server) DynamicFilter(ctx context.Context, in *pb.SearchRequest) (*pb.TypeResponse, error) {
	var bin = in.GetBin()
	var network = in.GetNetwork()
	var product = in.GetProductName()
	var issuer = in.GetIssuer()
	var country = in.GetCountry()
	var Type = in.GetType()

	ex := []goqu.Expression{}
	if network != "" {
		ex = append(ex, goqu.Ex{"network": network})
	} else if network == "NULL" {
		ex = append(ex, goqu.Ex{"network": nil})
	}

	if bin != 0 {
		ex = append(ex, goqu.L("start LIKE ?", strconv.Itoa(int(bin))+"%"))
	}

	if product != "" {
		ex = append(ex, goqu.Ex{"product_name": product})
	} else if product == "NULL" {
		ex = append(ex, goqu.Ex{"product_name": nil})
	}

	if issuer != "" {
		ex = append(ex, goqu.L("issuer LIKE ?", issuer+"%"))
	} else if issuer == "NULL" {
		ex = append(ex, goqu.Ex{"issuer": nil})
	}

	if country != "" {
		ex = append(ex, goqu.Ex{"country": country})
	} else if country == "NULL" {
		ex = append(ex, goqu.Ex{"country": nil})
	}

	if Type != "" {
		ex = append(ex, goqu.Ex{"type": Type})
	} else if Type == "NULL" {
		ex = append(ex, goqu.Ex{"type": nil})
	}
	networkquery, _, _ := dialect.From("bins").Select(goqu.L("IFNULL(network,'NULL')")).Where(ex...).GroupBy("network").ToSQL()
	selNetworks, err := db.Query(networkquery)
	var networks []string
	if err != nil {
		fmt.Println(networkquery)
		fmt.Println(network)
		fmt.Println(err)
	}

	defer selNetworks.Close()
	for selNetworks.Next() {
		var d string
		selNetworks.Scan(&d)
		networks = append(networks, d)
	}

	productquery, _, _ := dialect.From("bins").Select(goqu.L("IFNULL(product_name,'NULL')")).Where(ex...).GroupBy("product_name").ToSQL()
	selProducts, _ := db.Query(productquery)
	defer selProducts.Close()
	var products []string
	for selProducts.Next() {
		var d string
		selProducts.Scan(&d)
		products = append(products, d)
	}

	typequery, _, _ := dialect.From("bins").Select(goqu.L("IFNULL(type,'NULL')")).Where(ex...).GroupBy("type").ToSQL()
	selType, _ := db.Query(typequery)
	defer selType.Close()
	var types []string
	for selType.Next() {
		var d string
		selType.Scan(&d)
		types = append(types, d)
	}

	countryquery, _, _ := dialect.From("bins").Select(goqu.L("IFNULL(country,'NULL')")).Where(ex...).GroupBy("country").ToSQL()
	selCountry, _ := db.Query(countryquery)
	defer selCountry.Close()
	var countries []string
	for selCountry.Next() {
		var d string
		selCountry.Scan(&d)
		countries = append(countries, d)
	}

	return &pb.TypeResponse{
		Network: networks,
		Product: products,
		Type:    types,
		Country: countries,
	}, nil
}

func (s *server) DynamicBanks(ctx context.Context, in *pb.SearchRequest) (*pb.IssuerResponse, error) {
	var bin = in.GetBin()
	var network = in.GetNetwork()
	var product = in.GetProductName()
	var issuer = in.GetIssuer()
	var country = in.GetCountry()
	var Type = in.GetType()
	i := 0
	ex := []goqu.Expression{}
	if network != "" {
		i++
		ex = append(ex, goqu.Ex{"network": network})
	} else if network == "NULL" {
		i++
		ex = append(ex, goqu.Ex{"network": nil})
	}

	if product != "" {
		i++
		ex = append(ex, goqu.Ex{"product_name": product})
	} else if product == "NULL" {
		i++
		ex = append(ex, goqu.Ex{"product_name": nil})
	}

	if bin != 0 {
		ex = append(ex, goqu.L("start LIKE ?", strconv.Itoa(int(bin))+"%"))
	}

	if issuer != "" {
		ex = append(ex, goqu.L("issuer LIKE ?", issuer+"%"))
	} else if issuer == "NULL" {
		ex = append(ex, goqu.Ex{"issuer": nil})
	}

	if country != "" {
		i++
		ex = append(ex, goqu.Ex{"country": country})
	} else if country == "NULL" {
		i++
		ex = append(ex, goqu.Ex{"country": nil})
	}

	if Type != "" {
		i++
		ex = append(ex, goqu.Ex{"type": Type})
	} else if Type == "NULL" {
		i++
		ex = append(ex, goqu.Ex{"type": nil})
	}
	var qr string
	if i == 4 {
		qr, _, _ = dialect.From("bins").Select(goqu.L("IFNULL(issuer,'null')")).Where(ex...).GroupBy("issuer").ToSQL()
	} else {
		qr, _, _ = dialect.From("bins").Limit(10).Select(goqu.L("IFNULL(issuer,'null')")).Where(ex...).GroupBy("issuer").ToSQL()
	}

	selIssuer, _ := db.Query(qr)
	defer selIssuer.Close()
	var issuers []string
	for selIssuer.Next() {
		var d string
		selIssuer.Scan(&d)
		issuers = append(issuers, d)
	}
	return &pb.IssuerResponse{
		Issuer: issuers,
	}, nil
}

func (s *server) DynamicFilter_6Digit(ctx context.Context, in *pb.Search6DigitRequest) (*pb.TypeResponse, error) {
	var bin = in.GetBin()
	var network = in.GetNetwork()
	var product = in.GetProductName()
	var issuer = in.GetIssuer()
	var country = in.GetCountry()
	var Type = in.GetType()

	ex := []goqu.Expression{}
	if network != "" {
		ex = append(ex, goqu.Ex{"network": network})
	} else if network == "NULL" {
		ex = append(ex, goqu.Ex{"network": nil})
	}

	if bin != 0 {
		ex = append(ex, goqu.L("bin LIKE ?", strconv.Itoa(int(bin))+"%"))
	}

	if product != "" {
		ex = append(ex, goqu.Ex{"product": product})
	} else if product == "NULL" {
		ex = append(ex, goqu.Ex{"product": nil})
	}

	if issuer != "" {
		ex = append(ex, goqu.L("issuer LIKE ?", issuer+"%"))
	} else if issuer == "NULL" {
		ex = append(ex, goqu.Ex{"issuer": nil})
	}

	if country != "" {
		ex = append(ex, goqu.Ex{"country": country})
	} else if country == "NULL" {
		ex = append(ex, goqu.Ex{"country": nil})
	}

	if Type != "" {
		ex = append(ex, goqu.Ex{"type": Type})
	} else if Type == "NULL" {
		ex = append(ex, goqu.Ex{"type": nil})
	}
	networkquery, _, _ := dialect.From("oldbins").Select(goqu.L("IFNULL(network,'NULL')")).Where(ex...).GroupBy("network").ToSQL()
	selNetworks, err := db.Query(networkquery)
	var networks []string
	if err != nil {
		fmt.Println(networkquery)
		fmt.Println(network)
		fmt.Println(err)
	}

	defer selNetworks.Close()
	for selNetworks.Next() {
		var d string
		selNetworks.Scan(&d)
		networks = append(networks, d)
	}

	productquery, _, _ := dialect.From("oldbins").Select(goqu.L("IFNULL(product,'NULL')")).Where(ex...).GroupBy("product").ToSQL()
	selProducts, _ := db.Query(productquery)
	defer selProducts.Close()
	var products []string
	for selProducts.Next() {
		var d string
		selProducts.Scan(&d)
		products = append(products, d)
	}

	typequery, _, _ := dialect.From("oldbins").Select(goqu.L("IFNULL(type,'NULL')")).Where(ex...).GroupBy("type").ToSQL()
	selType, _ := db.Query(typequery)
	defer selType.Close()
	var types []string
	for selType.Next() {
		var d string
		selType.Scan(&d)
		types = append(types, d)
	}

	countryquery, _, _ := dialect.From("oldbins").Select(goqu.L("IFNULL(country,'NULL')")).Where(ex...).GroupBy("country").ToSQL()
	selCountry, _ := db.Query(countryquery)
	defer selCountry.Close()
	var countries []string
	for selCountry.Next() {
		var d string
		selCountry.Scan(&d)
		countries = append(countries, d)
	}

	return &pb.TypeResponse{
		Network: networks,
		Product: products,
		Type:    types,
		Country: countries,
	}, nil
}
func (s *server) DynamicBanks_6Digit(ctx context.Context, in *pb.Search6DigitRequest) (*pb.IssuerResponse, error) {
	var bin = in.GetBin()
	var network = in.GetNetwork()
	var product = in.GetProductName()
	var issuer = in.GetIssuer()
	var country = in.GetCountry()
	var Type = in.GetType()
	i := 0
	ex := []goqu.Expression{}
	if network != "" {
		i++
		ex = append(ex, goqu.Ex{"network": network})
	} else if network == "NULL" {
		i++
		ex = append(ex, goqu.Ex{"network": nil})
	}

	if product != "" {
		i++
		ex = append(ex, goqu.Ex{"product": product})
	} else if product == "NULL" {
		i++
		ex = append(ex, goqu.Ex{"product": nil})
	}

	if bin != 0 {
		ex = append(ex, goqu.L("bin LIKE ?", strconv.Itoa(int(bin))+"%"))
	}

	if issuer != "" {
		ex = append(ex, goqu.L("issuer LIKE ?", issuer+"%"))
	} else if issuer == "NULL" {
		ex = append(ex, goqu.Ex{"issuer": nil})
	}

	if country != "" {
		i++
		ex = append(ex, goqu.Ex{"country": country})
	} else if country == "NULL" {
		i++
		ex = append(ex, goqu.Ex{"country": nil})
	}

	if Type != "" {
		i++
		ex = append(ex, goqu.Ex{"type": Type})
	} else if Type == "NULL" {
		i++
		ex = append(ex, goqu.Ex{"type": nil})
	}
	var qr string
	if i == 4 {
		qr, _, _ = dialect.From("oldbins").Select(goqu.L("IFNULL(issuer,'null')")).Where(ex...).GroupBy("issuer").ToSQL()
	} else {
		qr, _, _ = dialect.From("oldbins").Limit(10).Select(goqu.L("IFNULL(issuer,'null')")).Where(ex...).GroupBy("issuer").ToSQL()
	}

	selIssuer, err := db.Query(qr)
	if err != nil {
		fmt.Println(qr)
		fmt.Println(err)
		return &pb.IssuerResponse{
			Issuer: []string{},
		}, nil

	}
	defer selIssuer.Close()
	var issuers []string
	for selIssuer.Next() {
		var d string
		selIssuer.Scan(&d)
		issuers = append(issuers, d)
	}
	return &pb.IssuerResponse{
		Issuer: issuers,
	}, nil
}
func (s *server) Log(ctx context.Context, in *pb.BinRequest) (*pb.Message, error) {
	if in.Incognito {
		return &pb.Message{
			Status:  true,
			Message: "OK",
		}, nil
	}
	go func(data *pb.BinRequest, context context.Context) {
		if data.Bin >= 100000 {
			selBins, _ := db.Query("SELECT start FROM bins WHERE RPAD(?,11,0) BETWEEN start AND end", data.Bin)
			defer selBins.Close()
			if selBins.Next() {
				var bin int64
				selBins.Scan(&bin)

				t := time.Now().UnixNano() / int64(time.Millisecond)
				var has *sql.Rows
				var err error
				geoinfo := helpers.IpDetails(context)

				user_id := helpers.Tokentouser(context)
				if user_id != nil {
					has, err = db.Query("SELECT bin FROM search_history where bin = ? and user_id = UUID_TO_BIN(?) and searched_at >= ?", bin, user_id, t-86400000)
				} else {
					has, err = db.Query("SELECT bin FROM search_history where bin = ? and ip = ? and searched_at >= ?", bin, geoinfo.Ip, t-86400000)
				}
				if err != nil {
					fmt.Println(err)
				}

				defer has.Close()
				if !has.Next() {

					qr, _, _ := dialect.Insert("search_history").Rows(
						goqu.Record{
							"uuid":        goqu.L("UUID_TO_BIN(?)", uuid.New().String()),
							"user_id":     goqu.L("UUID_TO_BIN(?)", user_id),
							"bin":         bin,
							"ip":          geoinfo.Ip,
							"country":     helpers.Getnull(geoinfo.Country),
							"state":       helpers.Getnull(geoinfo.State),
							"city":        helpers.Getnull(geoinfo.City),
							"action":      in.From,
							"searched_at": t,
						},
					).ToSQL()
					if err := globalcache.Publish("query", qr).Err(); err != nil {
						fmt.Println(qr)
						fmt.Println(err)
					}
				}

			}

		}
	}(in, ctx)

	return &pb.Message{
		Status:  true,
		Message: "OK",
	}, nil
}

func (s *server) GetToken(ctx context.Context, in *pb.TokenRequest) (*pb.TokenResponse, error) {
	if in.Token == "" {
		return &pb.TokenResponse{
			Message: &pb.Message{
				Message: "Token is empty",
				Status:  false,
			},
		}, nil
	}

	client, err := firebaseApp.Auth(ctx)
	if err != nil {
		fmt.Println(err)
		return &pb.TokenResponse{
			Message: &pb.Message{
				Message: "Something went wrong while fetching user",
				Status:  false,
			},
		}, nil
	}

	authToken, err := client.VerifyIDToken(ctx, in.Token)

	if err != nil {
		fmt.Println(err)
		return &pb.TokenResponse{
			Message: &pb.Message{
				Message: "Unable to verify token",
				Status:  false,
			},
		}, nil
	}

	u, err := client.GetUser(ctx, authToken.UID)

	if err != nil {
		fmt.Println(err)
		return &pb.TokenResponse{
			Message: &pb.Message{
				Message: "Something went wrong while fetching user",
				Status:  false,
			},
		}, nil

	}
	if u.Email != in.Email {
		return &pb.TokenResponse{
			Message: &pb.Message{
				Message: "Email does not match",
				Status:  false,
			},
		}, nil
	}

	user, err := db.Query("SELECT BIN_TO_UUID(user_id) as user_id FROM users WHERE email = ?", in.Email)
	if err != nil {
		fmt.Println(err)
		return &pb.TokenResponse{
			Message: &pb.Message{
				Message: "Something went wrong while fetching user",
				Status:  false,
			},
		}, nil

	}
	defer user.Close()
	var user_id string

	if user.Next() {
		user.Scan(&user_id)
		token, _ := helpers.GenerateToken(user_id)
		qr, _, _ := dialect.Update("users").Set(goqu.Record{
			"image":      in.Image,
			"locate":     in.Locate,
			"updated_at": time.Now().UnixNano() / int64(time.Millisecond),
		}).Where(goqu.C("email").Eq(in.Email)).ToSQL()
		_, err := db.Query(qr)
		if err != nil {
			fmt.Println(err)
			return &pb.TokenResponse{
				Message: &pb.Message{
					Message: "Something went wrong while updating user",
					Status:  false,
				},
			}, nil

		}
		return &pb.TokenResponse{
			Message: &pb.Message{
				Message: "Logged in as " + in.Email,
				Status:  true,
			},
			Token: token,
		}, nil

	} else {
		user_id = uuid.New().String()
		token, _ := helpers.GenerateToken(user_id)
		p, _ := peer.FromContext(ctx)
		geoinfo := helpers.Ip{
			Ip: helpers.IptoDecimal(strings.Split(p.Addr.String(), ":")[0]),
		}

		qr, _, _ := dialect.Insert("users").Rows(goqu.Record{
			"user_id":    goqu.L("UUID_TO_BIN(?)", helpers.Getnull(user_id)),
			"email":      helpers.Getnull(in.Email),
			"name":       helpers.Getnull(in.Name),
			"image":      helpers.Getnull(in.Image),
			"country":    helpers.Getnull(geoinfo.Country),
			"state":      helpers.Getnull(geoinfo.State),
			"city":       helpers.Getnull(geoinfo.City),
			"ip":         geoinfo.Ip,
			"locate":     helpers.Getnull(in.Locate),
			"created_at": time.Now().UnixNano() / int64(time.Millisecond),
			"updated_at": time.Now().UnixNano() / int64(time.Millisecond),
		}).ToSQL()
		_, err = db.Query(qr)
		if err != nil {
			fmt.Println(err)
			return &pb.TokenResponse{
				Message: &pb.Message{
					Message: "Something went wrong while creating user",
					Status:  false,
				},
			}, nil

		}
		return &pb.TokenResponse{
			Message: &pb.Message{
				Message: "Signed up successfully",
				Status:  false,
			},
			Token: token,
		}, nil
	}
}

func (s *server) GetImage(ctx context.Context, in *pb.ImageRequest) (*pb.ImageResponse, error) {
	return &pb.ImageResponse{
		Image:  image,
		Source: imageSource,
	}, nil
}

func (s *server) IsPremium(ctx context.Context, in *pb.IsPremiumRequest) (*pb.IsPremiumResponse, error) {
	user_id := helpers.Tokentouser(ctx)
	if user_id == nil {
		return &pb.IsPremiumResponse{
			Premium: false,
			Expire:  0,
		}, nil
	}
	qr, _, _ := dialect.From("users").Select("pro_expire").Where(goqu.C("user_id").Eq(goqu.L("UUID_TO_BIN(?)", user_id))).ToSQL()
	sel, err := db.Query(qr)
	if err != nil {
		return &pb.IsPremiumResponse{
			Premium: false,
			Expire:  0,
		}, nil
	}
	if !sel.Next() {
		return &pb.IsPremiumResponse{
			Premium: false,
			Expire:  0,
		}, nil
	}
	var pro_expire int64
	sel.Scan(&pro_expire)

	if pro_expire < time.Now().UnixMilli() {
		return &pb.IsPremiumResponse{
			Premium: false,
			Expire:  0,
		}, nil
	}
	return &pb.IsPremiumResponse{
		Premium: true,
		Expire:  pro_expire,
	}, nil
}
func (s *server) GetHistory(ctx context.Context, in *pb.HistoryRequest) (*pb.HistoryResponse, error) {
	user_id := helpers.Tokentouser(ctx)
	if user_id != nil {
		selectQuery, _ := db.Query("SELECT bin,searched_at FROM search_history WHERE user_id = UUID_TO_BIN(?) ORDER BY searched_at DESC", user_id)
		defer selectQuery.Close()
		var data []*pb.Stat
		for selectQuery.Next() {
			d := &pb.Stat{}
			selectQuery.Scan(&d.Bin, &d.At)
			data = append(data, d)
		}
		return &pb.HistoryResponse{
			History: data,
		}, nil
	} else {
		return &pb.HistoryResponse{}, nil
	}
}

func (s *server) GetFav(ctx context.Context, in *pb.FavRequest) (*pb.FavList, error) {
	user_id := helpers.Tokentouser(ctx)
	if user_id != nil {
		row, err := db.Query("SELECT bins FROM users WHERE user_id = UUID_TO_BIN(?)", user_id)
		if err != nil {
			fmt.Println(err)
		}
		defer row.Close()
		var bins json.RawMessage
		var binsint []int64
		if row.Next() {
			if err := row.Scan(&bins); err != nil {
				fmt.Println(err)
			}
			var binsstr []string
			json.Unmarshal(bins, &binsstr)

			for _, bin := range binsstr {
				binint, err := strconv.ParseInt(bin, 10, 64)
				if err != nil {
					fmt.Println(err)

				}
				binsint = append(binsint, binint)
			}

		}

		return &pb.FavList{
			Bin: binsint,
		}, nil
	} else {
		return &pb.FavList{}, nil
	}
}
