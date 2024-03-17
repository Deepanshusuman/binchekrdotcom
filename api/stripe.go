package api

import (
	"binchecker/credential"
	"fmt"

	"github.com/stripe/stripe-go/v75"
	"github.com/stripe/stripe-go/v75/balancetransaction"
	"github.com/stripe/stripe-go/v75/charge"
	"github.com/stripe/stripe-go/v75/checkout/session"
	"github.com/stripe/stripe-go/v75/customer"
	"github.com/stripe/stripe-go/v75/paymentintent"
)

func GetPaymentURL(amount int64, customerId string, currency string, payment_id string) (*string, error) {
	stripe.Key = credential.STRIPE_SECRET
	s, err := session.New(&stripe.CheckoutSessionParams{
		Customer: &customerId,
		Metadata: map[string]string{
			"payment_id": payment_id,
		},
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   &currency,
					UnitAmount: &amount,

					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String("Add Balance"),
						Description: stripe.String("Balance for site.com"),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},

		CancelURL:  stripe.String("https://site.com/cancelled"),
		Mode:       stripe.String("payment"),
		SuccessURL: stripe.String("https://site.com/success"),
	})

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return &s.URL, nil
}

func CreateCustomer(email string, name string) (string, error) {
	stripe.Key = credential.STRIPE_SECRET
	customer, err := customer.New(&stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	})
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	return customer.ID, nil
}

func DeleteCustomer(customer_id string) error {
	stripe.Key = credential.STRIPE_SECRET
	_, err := customer.Del(customer_id, &stripe.CustomerParams{})
	if err != nil {
		fmt.Println(err)
	}
	return err
}

// get payment intent
func GetPaymentIntentDetails(payment_intent string) (*stripe.PaymentIntent, error) {
	stripe.Key = credential.STRIPE_SECRET
	transaction, err := paymentintent.Get(payment_intent, &stripe.PaymentIntentParams{})
	if err != nil {
		fmt.Println(err)
		return &stripe.PaymentIntent{}, err
	}
	return transaction, nil
}

// get charges
func GetCharges(payment_intent *stripe.PaymentIntent) (int64, string, error) {
	stripe.Key = credential.STRIPE_SECRET

	charges, err := charge.Get(payment_intent.LatestCharge.ID, &stripe.ChargeParams{})
	if err != nil {
		fmt.Println(err)
		return 0, "", err

	}
	balanceTranx, err := balancetransaction.Get(charges.BalanceTransaction.ID, &stripe.BalanceTransactionParams{})
	if err != nil {
		fmt.Println(err)
		return 0, "", err
	}

	return balanceTranx.Fee, string(balanceTranx.Currency), nil
}
