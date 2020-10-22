package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	utils "github.com/bostontrader/okcommon"
	"github.com/bostontrader/okconnect/compare"
	"github.com/bostontrader/okconnect/config"
	"github.com/gojektech/heimdall/httpclient"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

/*
In this test scenario, we're going to figure out how to persuade OKConnect to place an order to sell 1 BTC and buy 25 LTC at the implied price of LTCBTC = 0.04 and then to make the trade.  We will then withdraw LTC from the exchange.  We will work our way through the entire life-cycle of doing this including:

  * The initial equity transaction to record the user's BTC asset on the user's books
  * The deposit of BTC with the OKCatbox
  * Transfer BTC from the OKCatbox funding account to the spot market
  * Place an order to trade BTC for LTC
  * Execute the order
  * Transfer LTC from the OKCatbox spot market to the funding account
  * Withdraw LTC from the OKCatbox.

In order to do this we're going to have to manage two sets of bookkeeping records.  We will do so by using [a publicly available Bookwerx Core server](https://github.com/bostontrader/bookwerx-core-rust).  The first set of books is for the OKCatbox itself.  In order to do its thing it needs to know things such as customer balances, hence the need for bookkeeping.  The second set of books is for the user acting as a customer of the OKCatbox.

Both sets of books should be constantly in sync.  We will use OKConnect to verify that said balances are in agreement as well as to take any action that affects both sets of books.
*/
func main() {

	// 1. This test is going to use two servers with two URLs and we'll also need an http client.
	BwServerUrl := "http://185.183.96.73:3003"
	CatboxURL := "http://localhost:8090"

	timeout := 30000 * time.Millisecond
	httpClient := httpclient.NewClient(httpclient.WithHTTPTimeout(timeout))

	// 2. Install, configure, and execute the OKCatbox

	// 2.1 Using the demo Bookwerx server, get credentials for the OKCatbox and setup some demo data.  Recall that this is the bookkeeping configuration that the OKCatbox uses for its own personal consumption.
	BookwerxCBAPIKey := POST_BW_Credentials(httpClient, BwServerUrl)
	fmt.Printf("BookwerxCBAPIKey=%s\n", BookwerxCBAPIKey)

	// 2.2 The OKCatbox will support these currencies...
	CurrencyBTC := POST_BW_LID(httpClient, fmt.Sprintf("%s/currencies", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&symbol=BTC&title=Bitcoin", BookwerxCBAPIKey))

	CurrencyLTC := POST_BW_LID(httpClient, fmt.Sprintf("%s/currencies", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&symbol=LTC&title=Litecoin", BookwerxCBAPIKey))

	// 2.3 The OKCatbox will need a hot wallet asset account for each of the supported currencies.
	HotWalletBTC := POST_BW_LID(httpClient, fmt.Sprintf("%s/accounts", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&currency_id=%d&title=Hot wallet", BookwerxCBAPIKey, CurrencyBTC))

	HotWalletLTC := POST_BW_LID(httpClient, fmt.Sprintf("%s/accounts", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&currency_id=%d&title=Hot wallet", BookwerxCBAPIKey, CurrencyLTC))

	// 2.4 The OKCatbox will need these customary categories in order to produce balance sheets and income statements.
	//CAT_ASSETS := POST_BW_LID(httpClient, fmt.Sprintf(
	//"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=A&title=Assets", BookwerxCBAPIKey))

	//CAT_LIABILITIES="$(curl -s -d "apikey=$BookwerxCBAPIKey&symbol=L&title=Liabilities" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_EQUITY="$(curl -s -d "apikey=$BookwerxCBAPIKey&symbol=Eq&title=Equity" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_REVENUE="$(curl -s -d "apikey=$BookwerxCBAPIKey&symbol=R&title=Revenue" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_EXPENSES="$(curl -s -d "apikey=$BookwerxCBAPIKey&symbol=Ex&title=Expenses" $BwServerUrl/categories | jq .LastInsertId)"

	// 2.4.1 The OKCatbox will also need to tag customer accounts for funding, spot available, and spot hold. Said accounts will be created by the OKCatbox later, when required.  But we want the categories defined now.
	CAT_FUNDING := POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=F&title=Funding", BookwerxCBAPIKey))

	CAT_SPOT_AVAILABLE := POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=SA&title=Spot available", BookwerxCBAPIKey))

	CAT_SPOT_HOLD := POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=SH&title=Spot hold", BookwerxCBAPIKey))

	// 2.4.2 Any hot wallet accounts shall be tagged with this category..."
	CAT_HOT_WALLET := POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=H&title=Hot wallet", BookwerxCBAPIKey))

	// Tag each hot wallet account as an Asset and a Hot Wallet.  We don't care about the return value.
	//_ = POST_BW_LID(httpClient, fmt.Sprintf(
	//"%s/acctcats", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&category_id=%d", BookwerxCBAPIKey, HotWalletBTC, CAT_ASSETS))

	//_ = POST_BW_LID(httpClient, fmt.Sprintf(
	//"%s/acctcats", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&category_id=%d", BookwerxCBAPIKey, HotWalletLTC, CAT_ASSETS))

	_ = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/acctcats", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&category_id=%d", BookwerxCBAPIKey, HotWalletBTC, CAT_HOT_WALLET))

	_ = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/acctcats", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&category_id=%d", BookwerxCBAPIKey, HotWalletLTC, CAT_HOT_WALLET))

	// 2.5 Build a config file for okcatbox.  You can see that some of the categories are duplicated.  Fix this.
	m := make(map[string]AH)
	m["1"] = AH{
		Available: CAT_SPOT_AVAILABLE,
		Hold:      CAT_SPOT_HOLD,
	}
	m["6"] = AH{
		Available: CAT_FUNDING,
		Hold:      0, // No Hold variation for funding
	}

	catboxConfig := Config{
		Bookwerx: Bookwerx{
			APIKey:           BookwerxCBAPIKey,
			Server:           BwServerUrl,
			FundingCat:       CAT_FUNDING,
			SpotAvailableCat: CAT_SPOT_AVAILABLE,
			SpotHoldCat:      CAT_SPOT_HOLD,
			HotWalletCat:     CAT_HOT_WALLET,
			TransferCats:     m,
		},
		ListenAddr: ":8090",
	}

	out, err := yaml.Marshal(catboxConfig)
	if err != nil {
		fmt.Printf("Error marshalling catbox config: err=%v\n", err)
		os.Exit(1)
	}
	err = ioutil.WriteFile("okcatbox.yaml", out, 0600)
	if err != nil {
		fmt.Printf("Error writing okcatbox config to okcatbox.yaml: err=%v\n", err)
		os.Exit(1)
	}
	//fmt.Printf("OKCatbox config=\n%#v\n", catboxConfig)
	catboxConfigS, _ := json.MarshalIndent(catboxConfig, "", "  ")
	fmt.Printf("OKCatbox config=\n%s\n\n", string(catboxConfigS))

	// 2.6 Start the okcatbox daemonized
	//okcatbox -config=okcatbox.yaml &
	cmd := exec.Command("okcatbox", "-config=okcatbox.yaml")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()

	// 3. Now setup the test monkey user.

	// 3.1 In the beginning... The user has nothing.  He must first establish his own account with the Bookwerx Core server.
	//TMU_APIKEY="$(curl -s -X POST $BwServerUrl/apikeys | jq -r .apikey)"
	TMU_APIKEY := POST_BW_Credentials(httpClient, BwServerUrl)
	fmt.Printf("TMU_APIKEY=%s\n", TMU_APIKEY)

	// 3.2 Since we are going to use BTC and LTC in our subsequent transactions, we must define them as currencies in Bookwerx. We have already done this for the OKCatbox books, but we are using the same currencies for the user's books and we must define them separately there."
	CurrencyBTC = POST_BW_LID(httpClient, fmt.Sprintf("%s/currencies", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&symbol=BTC&title=Bitcoin", TMU_APIKEY))

	//CurrencyLTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&symbol=LTC&title=Litecoin" $BwServerUrl/currencies | jq .LastInsertId)"
	//CurrencyLTC := POST_BW_LID(httpClient, fmt.Sprintf("%s/currencies", TMU_APIKEY), fmt.Sprintf("apikey=%s&rarity=0&symbol=LTC&title=Litecoin", TMU_APIKEY))

	// 3.3 Establish some necessary bookkeeping accounts for the user.  Notice that several of the accounts have identical titles.  They are differentiated according to their currencies.

	// 3.3.1 We must have owner's equity to get the party started.
	ACCT_EQUITY := POST_BW_LID(httpClient, fmt.Sprintf("%s/accounts", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&currency_id=%d&title=Owner's equity", TMU_APIKEY, CurrencyBTC))

	// 3.3.2 We must have asset accounts for our local wallets.
	ACCT_LCL_WALLET_BTC := POST_BW_LID(httpClient, fmt.Sprintf("%s/accounts", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&currency_id=%d&title=Local wallet", TMU_APIKEY, CurrencyBTC))

	//ACCT_LCL_WALLET_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CurrencyLTC&title=Local Wallet" $BwServerUrl/accounts | jq .LastInsertId)"

	// 3.3.3 We must have asset accounts for our funding accounts on OKEx
	ACCT_FUNDING_BTC := POST_BW_LID(httpClient, fmt.Sprintf("%s/accounts", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&currency_id=%d&title=OKEx Funding", TMU_APIKEY, CurrencyBTC))
	//ACCT_FUNDING_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CurrencyLTC&title=OKEx Funding" $BwServerUrl/accounts | jq .LastInsertId)"

	// 3.3.4 We must have asset accounts for our balances in the spot trading area of OKEx.  Not merely one, but two balances, available and amounts on hold.
	//ACCT_SPOT_AVAIL_BTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CurrencyBTC&title=OKEx Spot- Available" $BwServerUrl/accounts | jq .LastInsertId)"
	//ACCT_SPOT_AVAIL_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CurrencyLTC&title=OKEx Spot- Available" $BwServerUrl/accounts | jq .LastInsertId)"
	//ACCT_SPOT_HOLD_BTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CurrencyBTC&title=OKEx Spot- Hold" $BwServerUrl/accounts | jq .LastInsertId)"
	//ACCT_SPOT_HOLD_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CurrencyLTC&title=OKEx Spot- Hold" $BwServerUrl/accounts | jq .LastInsertId)"

	// 3.3.5 We will need expense accounts for each currency for the variety of fees that we will encounter.
	//ACCT_FEE_BTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CurrencyBTC&title=Fee" $BwServerUrl/accounts | jq .LastInsertId)"
	//ACCT_FEE_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CurrencyLTC&title=Fee" $BwServerUrl/accounts | jq .LastInsertId)"

	// 3.4 Establish some necessary categories

	// 3.4.1 In order to produce Balance Sheet and Income Statement reports we must have these categories:
	//CAT_ASSETS="$(curl -s -d "apikey=$TMU_APIKEY&symbol=A&title=Assets" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_LIABILITIES="$(curl -s -d "apikey=$TMU_APIKEY&symbol=L&title=Liabilities" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_EQUITY="$(curl -s -d "apikey=$TMU_APIKEY&symbol=Eq&title=Equity" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_REVENUE="$(curl -s -d "apikey=$TMU_APIKEY&symbol=R&title=Revenue" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_EXPENSES="$(curl -s -d "apikey=$TMU_APIKEY&symbol=Ex&title=Expenses" $BwServerUrl/categories | jq .LastInsertId)"

	// 3.4.2 We will need a general ability to find all funding, spot-available, and spot-hold accounts so we need these categories.
	CAT_FUNDING = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=F&title=Funding", TMU_APIKEY))

	CAT_SPOT_AVAILABLE = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=SA&title=Spot available", TMU_APIKEY))

	CAT_SPOT_HOLD = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=SH&title=Spot hold", TMU_APIKEY))

	// 3.5 Now tag these accounts with suitable categories.  Just do it, we don't care about saving any return values.
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_LCL_WALLET_BTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_LCL_WALLET_LTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FUNDING_BTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FUNDING_LTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_AVAIL_BTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_AVAIL_LTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_HOLD_BTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats

	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FEE_BTC&category_id=$CAT_EXPENSES" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FEE_LTC&category_id=$CAT_EXPENSES" $BwServerUrl/acctcats

	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_EQUITY&category_id=$CAT_EQUITY" $BwServerUrl/acctcats

	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FUNDING_BTC&category_id=$CAT_FUNDING" $BwServerUrl/acctcats
	_ = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/acctcats", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&category_id=%d", TMU_APIKEY, ACCT_FUNDING_BTC, CAT_FUNDING))
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FUNDING_LTC&category_id=$CAT_FUNDING" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_AVAIL_BTC&category_id=$CAT_SPOT_AVAILABLE" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_AVAIL_LTC&category_id=$CAT_SPOT_AVAILABLE" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_HOLD_BTC&category_id=$CAT_SPOT_HOLD" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_HOLD_LTC&category_id=$CAT_SPOT_HOLD" $BwServerUrl/acctcats

	// 3.6 Get read, read-trade, and read-withdraw credentials from the OKCatbox for this user.  As with the real OKEx API we'll need access credentials.  This OKCatbox endpoint is a convenience to make it easy to get credentials.  The real OKEx server doesn't issue credentials via the API.
	USER_ID := "moe"

	// 3.6.1 read
	OKCATBOX_CREDENTIALS_FILE_READ := "okcatbox-read.json"
	cb_credentials_read := buildOKCatboxCredentials(httpClient, CatboxURL, CredentialsRequestBody{UserID: USER_ID, Type: "read"}, OKCATBOX_CREDENTIALS_FILE_READ)

	// 3.6.2 read-trade
	//OKCATBOX_CREDENTIALS_FILE_READ_TRADE := "okcatbox-read-trade.json"
	//cb_credentials_read := buildOKCatboxCredentials(httpClient, CatboxURL, CredentialsRequestBody{UserID: USER_ID, Type: "read"}, OKCATBOX_CREDENTIALS_FILE_READ)

	// 3.6.3 read-withdrawal
	//OKCATBOX_CREDENTIALS_FILE_READ_WITHDRAW := "okcatbox-read-withdraw.json"
	//cb_credentials_read := buildOKCatboxCredentials(httpClient, CatboxURL, CredentialsRequestBody{UserID: USER_ID, Type: "read"}, OKCATBOX_CREDENTIALS_FILE_READ)

	// parse this into json so we can access it later

	//OKCATBOX_CREDENTIALS_FILE_READ_TRADE=okcatbox-read-trade.json
	//curl -s -X POST $CatboxURL/catbox/credentials --data "{\"user_id\":\"$USER_ID\",\"type\":\"read-trade\"}" --output $OKCATBOX_CREDENTIALS_FILE_READ_TRADE

	// 4. Setup okconnect.
	//echo "bookwerxconfig:" > okconnect.yaml
	//echo "  apikey: $TMU_APIKEY" >> okconnect.yaml
	//echo "  server: $BwServerUrl" >> okconnect.yaml
	//echo "  funding_cat: $CAT_FUNDING" >> okconnect.yaml  // This is supposed to be a category in the user's books!
	//echo "  spot_available_cat: $CAT_SPOT_AVAILABLE" >> okconnect.yaml
	//echo "  spot_hold_cat: $CAT_SPOT_HOLD" >> okconnect.yaml
	//echo "okexconfig:" >> okconnect.yaml
	//echo "  credentials: $OKCATBOX_CREDENTIALS_FILE_READ" >> okconnect.yaml
	//echo "  server: $CatboxURL" >> okconnect.yaml
	//cat okconnect.yaml

	okconnect_cfg := config.Config{
		BookwerxConfig: config.BookwerxConfig{
			APIKey:           TMU_APIKEY,
			Server:           BwServerUrl,
			FundingCat:       CAT_FUNDING,
			SpotAvailableCat: CAT_SPOT_AVAILABLE,
			SpotHoldCat:      CAT_SPOT_AVAILABLE,
		},
		OKExConfig: config.OKExConfig{
			Credentials: OKCATBOX_CREDENTIALS_FILE_READ,
			Server:      CatboxURL,
		},
	}

	out, err = yaml.Marshal(okconnect_cfg)
	if err != nil {
		fmt.Printf("Error marshalling the okconnect config: err=%v\n", err)
		os.Exit(1)
	}
	err = ioutil.WriteFile("okconnect.yaml", out, 0600)
	if err != nil {
		fmt.Printf("Error writing the okconnect config to okconnect.yaml: err=%v\n", err)
		os.Exit(1)
	}
	//fmt.Printf("Test monkey's bookwerx config=\n%#v\n", okconnect_cfg)
	okconnectConfigS, _ := json.MarshalIndent(okconnect_cfg, "", "  ")
	fmt.Printf("okconnect config=\n%s\n\n", string(okconnectConfigS))

	// 5. Initial equity for the TMU
	TXID := POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/transactions", BwServerUrl), fmt.Sprintf("apikey=%s&notes=Initial Equity&time=2020-05-01T12:34:55.000Z", TMU_APIKEY))
	_ = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/distributions", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&amount=2&amount_exp=0&transaction_id=%d", TMU_APIKEY, ACCT_LCL_WALLET_BTC, TXID))
	_ = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/distributions", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&amount=-2&amount_exp=0&transaction_id=%d", TMU_APIKEY, ACCT_EQUITY, TXID))

	// 6. Simulate the deposit of BTC into the funding account.  This is a tedious and difficult issue for a variety of reasons.  Therefore, at this point, we will use this convenience endpoint from the OKCatbox where we can easily assert a deposit. This OKCatbox endpoint is a convenience to make it easy to make deposits.  The real OKEx server doesn't manage deposits via the API.

	// 6.1 Make the deposit manually to the catbox. We use the cb_credentials_read merely to identify the user.
	_ = POST_Catbox_Deposit(httpClient, CatboxURL, fmt.Sprintf("&apikey=%s&currency_symbol=BTC&quan=1.5", cb_credentials_read.Key))

	// 6.2 Let's use okconnect to compare the user's balances in Bookwerx with the corresponding balances in the OKCatbox.  We should detect a discrepancy because the OKCatbox has a deposit,  but we haven't yet made a matching transaction on the user's books.
	out1, err := exec.Command("okconnect", "compare", "-config", "okconnect.yaml").Output()
	if err != nil {
		fmt.Printf("okconnect error 1: err=%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("okconnect output 1=%s\n", out1)
	comparison := make([]compare.Comparison, 0)
	dec := json.NewDecoder(bytes.NewReader(out1))
	dec.DisallowUnknownFields()
	err = dec.Decode(&comparison)
	if err != nil {
		fmt.Println("Comparison JSON decode error: %v", err)
		os.Exit(1)
	}
	fmt.Printf("Comparison struct =%#v\n", comparison)
	//if [ $(jq -r .[0].Category okconnect.out) != "F" ]; then echo "okconnect error"; exit 1; fi
	if comparison[0].Category != "F" {
		fmt.Println("Okconnect error")
		os.Exit(1)
	}

	//if [ $(jq -r .[0].OKExBalance.Balance okconnect.out) != "1.5" ]; then echo "okconnect error"; exit 1; fi
	//if comparison[0].Category != "F" {
	//fmt.Println("Okconnect error")
	//os.Exit(1)
	//}

	//if [ $(jq -r .[0].BookwerxBalance.Nil okconnect.out) != "true" ]; then echo "okconnect error"; exit 1; fi

	// 6.3 Now create the bookwerx transaction on our user's books.
	TXID = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/transactions", BwServerUrl), fmt.Sprintf("apikey=%s&notes=Xfer BTC to OKEx&time=2020-05-01T12:34:55.000Z", TMU_APIKEY))
	_ = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/distributions", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&amount=15&amount_exp=-1&transaction_id=%d", TMU_APIKEY, ACCT_FUNDING_BTC, TXID))
	_ = POST_BW_LID(httpClient, fmt.Sprintf(
		"%s/distributions", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&amount=-15&amount_exp=-1&transaction_id=%d", TMU_APIKEY, ACCT_LCL_WALLET_BTC, TXID))

	// 6.4 Let's use okconnect again to compare the user's balances in Bookwerx with the corresponding balances in the OKCatbox. Now there should be zero discrepancies.
	// ./okconnect compare -config okconnect.yaml
	out2, err := exec.Command("okconnect", "compare", "-config", "okconnect.yaml").Output()
	if err != nil {
		fmt.Printf("okconnect error 2: err=%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("okconnect output 2=%s\n", out2)

	comparison = make([]compare.Comparison, 0)
	dec = json.NewDecoder(bytes.NewReader(out2))
	dec.DisallowUnknownFields()
	err = dec.Decode(&comparison)
	if err != nil {
		fmt.Println("Comparison JSON decode error: %v", err)
		os.Exit(1)
	}
	fmt.Printf("Comparison struct =%#v\n", comparison)
	//if [ $(jq -r .[0].Category okconnect.out) != "F" ]; then echo "okconnect error"; exit 1; fi
	if len(comparison) != 0 {
		fmt.Println("Okconnect error 1")
		os.Exit(1)
	}
	//if [ $(jq -r .[0] okconnect.out) != "null" ]; then echo "okconnect error"; exit 1; fi

	// 7. Things are going to start happening now!  The next step is to transfer some BTC from the funding account (6) into the spot market (1).  This is something that okconnect can easily do.

	//okconnect transfer -currency BTC -quan 1.25 -from 6 -to 1 -config okconnect.yaml

}

func POST(client *httpclient.Client, url string, body io.Reader, headers http.Header) []byte {

	resp, err := client.Post(url, body, headers)
	if err != nil {
		fmt.Printf("Cannot POST to %s: %v\n", url, err)
		os.Exit(1)
	}

	response_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading from POST response: URL=%s, err=%v\n", url, err)
		os.Exit(1)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("Status code error: Expected status=200, Received=%d, URL=%s\nbody=%s\n", resp.StatusCode, url, string(response_body))
		os.Exit(1)
	}

	return response_body
}

// When the OKCatbox executes it needs some configuration.
// Duplicated from github.com/bostontrader/okcatbox.  Factor this out.
type Config struct {
	Bookwerx   Bookwerx
	ListenAddr string
}

// The OKCatbox will use a Bookwerx server for its internal operation.
// Duplicated from github.com/bostontrader/okcatbox, but changed int32 to uint32.  Factor this out.
type Bookwerx struct {
	APIKey string
	Server string

	// Customer accounts shall be tagged with these categories where applicable.
	// These are deprecated.  Use TransferCats instead.
	FundingCat       uint32 `yaml:"funding_cat"`
	SpotAvailableCat uint32 `yaml:"spot_available_cat"`
	SpotHoldCat      uint32 `yaml:"spot_hold_cat"`

	// Any hot wallet shall be tagged with this category.
	HotWalletCat uint32 `yaml:"hot_wallet_cat"`

	// The OKEx API specifies transfer endpoints using strings.  ie. "1" = spot, "6" = funding, etc.
	TransferCats map[string]AH `yaml:"transfer_cats"`
}

// A transfer requires a source and a destination (from,to).  Each endpoint has an available and hold amount.
// Here we store the bookwerx category_ids for a particular transfer endpoint.
// Duplicated from github.com/bostontrader/okcatbox, but changed int32 to uint32.  Factor this out.
type AH struct {
	Available uint32
	Hold      uint32
}

type LID struct {
	LastInsertID uint32
}

// This is the configuration for a bookwerx core server and apikey for an ordinary user.

// Duplicated from github.com/bostontrader/okcatbox.  Factor this out.
type CredentialsRequestBody struct {
	UserID string `json:"user_id"`
	Type   string
}

// Duplicated from github.com/bostontrader/okcatbox.  Factor this out.
type User struct {
	UserID string
}

func POST_BW_Credentials(httpClient *httpclient.Client, baseURL string) string {

	url := fmt.Sprintf("%s/apikeys", baseURL)
	response_body := POST(httpClient, url, nil, nil)

	type BW_APIKEY struct {
		APIKEY string `json:"apikey"`
	}

	var BookwerxCBAPIKey BW_APIKEY
	dec := json.NewDecoder(bytes.NewReader(response_body))
	err := dec.Decode(&BookwerxCBAPIKey)
	if err != nil {
		fmt.Printf("JSON Decode error: Body=%s, err=%v\n", string(response_body), err)
		os.Exit(1)
	}

	return BookwerxCBAPIKey.APIKEY
}

func POST_Catbox_Credentials(httpClient *httpclient.Client, baseURL string, credentialsRequestBody CredentialsRequestBody) utils.Credentials {

	url := fmt.Sprintf("%s/catbox/credentials", baseURL)
	h := make(map[string][]string)
	h["Content-Type"] = []string{"application/json"}
	b, err := json.Marshal(credentialsRequestBody)
	if err != nil {
		fmt.Printf("JSON Encode error: Obj=%v, err=%v\n", credentialsRequestBody, err)
		os.Exit(1)
	}
	response_body := POST(httpClient, url, bytes.NewReader(b), h)

	var credentials utils.Credentials
	dec := json.NewDecoder(bytes.NewReader(response_body))
	err = dec.Decode(&credentials)
	if err != nil {
		fmt.Printf("JSON Decode error: Body=%s, err=%v\n", string(response_body), err)
		os.Exit(1)
	}

	return credentials
}

func POST_Catbox_Deposit(httpClient *httpclient.Client, baseURL string, body string) []byte {

	url := fmt.Sprintf("%s/catbox/deposit", baseURL)
	h := make(map[string][]string)
	h["Content-Type"] = []string{"application/x-www-form-urlencoded"}
	response_body := POST(httpClient, url, strings.NewReader(body), h)

	return response_body
}

// Post to bookwerx and get a Last Insert ID back.
func POST_BW_LID(httpClient *httpclient.Client, url string, body string) uint32 {

	h := make(map[string][]string)
	h["Content-Type"] = []string{"application/x-www-form-urlencoded"}
	response_body := POST(httpClient, url, strings.NewReader(body), h)

	var lid LID
	dec := json.NewDecoder(bytes.NewReader(response_body))
	err := dec.Decode(&lid)
	if err != nil {
		fmt.Printf("JSON Decode error: Body=%s, err=%v\n", string(response_body), err)
		os.Exit(1)
	}

	return lid.LastInsertID
}

// A MaybeBalance simulates a Maybe.
// Duplicated from github.com/bostontrader/okcatbox.  Factor this out.
type MaybeBalance struct {
	Balance decimal.Decimal
	Nil     bool // Is the balance really supposed to be nil?
}

func buildOKCatboxCredentials(httpClient *httpclient.Client, baseURL string, credentialsRequestBody CredentialsRequestBody, credentialsFileName string) utils.Credentials {
	methodName := "oktest:main.go:buildOKCatboxCredentials"

	cbc := POST_Catbox_Credentials(httpClient, baseURL, credentialsRequestBody)

	// Marshal these credentials to JSON and write to a file.
	out, err := json.Marshal(cbc)
	if err != nil {
		fmt.Printf("%s: JSON marshal error: Err=%v\nbody=%v\n", methodName, err, cbc)
		os.Exit(1)
	}
	err = ioutil.WriteFile(credentialsFileName, out, 0600)
	if err != nil {
		fmt.Printf("%s: Error writing okcatbox credentials to %s: err=%v\n", methodName, credentialsFileName, err)
		os.Exit(1)
	}

	return cbc
}
