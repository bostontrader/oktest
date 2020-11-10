package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	utils "github.com/bostontrader/okcommon"
	"github.com/bostontrader/okconnect/compare"
	"github.com/bostontrader/okconnect/config"
	"github.com/gojektech/heimdall/httpclient"
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

	timeout := 60000 * time.Millisecond
	httpClient := httpclient.NewClient(httpclient.WithHTTPTimeout(timeout))

	fmt.Printf("Section 1 success.  I have performed basic initialization.\n\n")

	// 2. Install, configure, and execute the OKCatbox

	// 2.1 Using the demo Bookwerx server, get credentials for the OKCatbox and setup some demo data.  Recall that this is the bookkeeping configuration that the OKCatbox uses for its own personal consumption.
	BookwerxCBAPIKey := PostBWCredentials(httpClient, BwServerUrl)
	fmt.Printf("BookwerxCBAPIKey=%s\n", BookwerxCBAPIKey)

	// 2.2 The OKCatbox will support these currencies...
	CurrencyBTC := PostBwLid(httpClient, fmt.Sprintf("%s/currencies", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&symbol=BTC&title=Bitcoin", BookwerxCBAPIKey))

	CurrencyLTC := PostBwLid(httpClient, fmt.Sprintf("%s/currencies", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&symbol=LTC&title=Litecoin", BookwerxCBAPIKey))

	// 2.3 The OKCatbox will need a hot wallet asset account for each of the supported currencies.
	HotWalletBTC := PostBwLid(httpClient, fmt.Sprintf("%s/accounts", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&currency_id=%d&title=Hot wallet", BookwerxCBAPIKey, CurrencyBTC))

	HotWalletLTC := PostBwLid(httpClient, fmt.Sprintf("%s/accounts", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&currency_id=%d&title=Hot wallet", BookwerxCBAPIKey, CurrencyLTC))

	// 2.4 The OKCatbox will need these customary categories in order to produce balance sheets and income statements.
	//CAT_ASSETS := PostBwLid(httpClient, fmt.Sprintf(
	//"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=A&title=Assets", BookwerxCBAPIKey))

	//CAT_LIABILITIES="$(curl -s -d "apikey=$BookwerxCBAPIKey&symbol=L&title=Liabilities" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_EQUITY="$(curl -s -d "apikey=$BookwerxCBAPIKey&symbol=Eq&title=Equity" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_REVENUE="$(curl -s -d "apikey=$BookwerxCBAPIKey&symbol=R&title=Revenue" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_EXPENSES="$(curl -s -d "apikey=$BookwerxCBAPIKey&symbol=Ex&title=Expenses" $BwServerUrl/categories | jq .LastInsertId)"

	// 2.5 The OKCatbox will need to tag customer accounts for funding, spot available, and spot hold. Said accounts will be created by the OKCatbox later, when required.  But we want the categories defined now.
	CatFunding := PostBwLid(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=F&title=Funding", BookwerxCBAPIKey))

	CatSpotAvailable := PostBwLid(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=SA&title=Spot available", BookwerxCBAPIKey))

	CatSpotHold := PostBwLid(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=SH&title=Spot hold", BookwerxCBAPIKey))

	// 2.6 The OKCatbox will need to tag transactions as deposits.
	CatDeposit := PostBwLid(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=DEP&title=Deposit", BookwerxCBAPIKey))

	// 2.7 Any hot wallet accounts shall be tagged with this category..."
	CatHotWallet := PostBwLid(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=H&title=Hot wallet", BookwerxCBAPIKey))

	// Tag each hot wallet account as an Asset and a Hot Wallet.  We don't care about the return value.
	//_ = PostBwLid(httpClient, fmt.Sprintf(
	//"%s/acctcats", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&category_id=%d", BookwerxCBAPIKey, HotWalletBTC, CAT_ASSETS))

	//_ = PostBwLid(httpClient, fmt.Sprintf(
	//"%s/acctcats", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&category_id=%d", BookwerxCBAPIKey, HotWalletLTC, CAT_ASSETS))

	_ = PostBwLid(httpClient, fmt.Sprintf(
		"%s/acctcats", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&category_id=%d", BookwerxCBAPIKey, HotWalletBTC, CatHotWallet))

	_ = PostBwLid(httpClient, fmt.Sprintf(
		"%s/acctcats", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&category_id=%d", BookwerxCBAPIKey, HotWalletLTC, CatHotWallet))

	// 2.8 Build a config file for okcatbox.  You can see that some of the categories are duplicated.  Fix this.
	m := make(map[string]AH)
	m["1"] = AH{
		Available: CatSpotAvailable,
		Hold:      CatSpotHold,
	}
	m["6"] = AH{
		Available: CatFunding,
		Hold:      0, // No Hold variation for funding
	}

	catboxConfig := Config{
		Bookwerx: Bookwerx{
			APIKey:           BookwerxCBAPIKey,
			Server:           BwServerUrl,
			CatDeposit:       CatDeposit,
			CatFunding:       CatFunding,
			CatHotWallet:     CatHotWallet,
			CatSpotAvailable: CatSpotAvailable,
			CatSpotHold:      CatSpotHold,
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
	_ = cmd.Start()

	fmt.Printf("Section 2 success.  I have configured and launched the catbox.\n\n")

	// 3. Now setup the test monkey user.

	// 3.1 In the beginning... The user has nothing.  He must first establish his own account with the Bookwerx Core server.
	TmuApiKey := PostBWCredentials(httpClient, BwServerUrl)
	fmt.Printf("TmuApiKey=%s\n", TmuApiKey)

	// 3.2 Since we are going to use BTC and LTC in our subsequent transactions, we must define them as currencies in Bookwerx. We have already done this for the OKCatbox books, but we are using the same currencies for the user's books and we must define them separately there."
	CurrencyBTC = PostBwLid(httpClient, fmt.Sprintf("%s/currencies", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&symbol=BTC&title=Bitcoin", TmuApiKey))

	//CurrencyLTC="$(curl -s -d "apikey=$TmuApiKey&rarity=0&symbol=LTC&title=Litecoin" $BwServerUrl/currencies | jq .LastInsertId)"
	//CurrencyLTC := PostBwLid(httpClient, fmt.Sprintf("%s/currencies", TmuApiKey), fmt.Sprintf("apikey=%s&rarity=0&symbol=LTC&title=Litecoin", TmuApiKey))
	fmt.Printf("Section 3.2 success.\n")

	// 3.3 Establish some necessary bookkeeping accounts for the user.  Notice that several of the accounts have identical titles.  They are differentiated according to their currencies.

	// 3.3.1 We must have owner's equity to get the party started.
	AcctEquity := PostBwLid(httpClient, fmt.Sprintf("%s/accounts", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&currency_id=%d&title=Owner's equity", TmuApiKey, CurrencyBTC))

	// 3.3.2 We must have asset accounts for our local wallets.
	AcctLocalWalletBTC := PostBwLid(httpClient, fmt.Sprintf("%s/accounts", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&currency_id=%d&title=Local wallet", TmuApiKey, CurrencyBTC))

	//ACCT_LCL_WALLET_LTC="$(curl -s -d "apikey=$TmuApiKey&rarity=0&currency_id=$CurrencyLTC&title=Local Wallet" $BwServerUrl/accounts | jq .LastInsertId)"

	// 3.3.3 We must have asset accounts for our funding accounts on OKEx
	AcctFundingBTC := PostBwLid(httpClient, fmt.Sprintf("%s/accounts", BwServerUrl), fmt.Sprintf("apikey=%s&rarity=0&currency_id=%d&title=OKEx Funding", TmuApiKey, CurrencyBTC))
	//ACCT_FUNDING_LTC="$(curl -s -d "apikey=$TmuApiKey&rarity=0&currency_id=$CurrencyLTC&title=OKEx Funding" $BwServerUrl/accounts | jq .LastInsertId)"

	// 3.3.4 We must have asset accounts for our balances in the spot trading area of OKEx.  Not merely one, but two balances, available and amounts on hold.
	//ACCT_SPOT_AVAIL_BTC="$(curl -s -d "apikey=$TmuApiKey&rarity=0&currency_id=$CurrencyBTC&title=OKEx Spot- Available" $BwServerUrl/accounts | jq .LastInsertId)"
	//ACCT_SPOT_AVAIL_LTC="$(curl -s -d "apikey=$TmuApiKey&rarity=0&currency_id=$CurrencyLTC&title=OKEx Spot- Available" $BwServerUrl/accounts | jq .LastInsertId)"
	//ACCT_SPOT_HOLD_BTC="$(curl -s -d "apikey=$TmuApiKey&rarity=0&currency_id=$CurrencyBTC&title=OKEx Spot- Hold" $BwServerUrl/accounts | jq .LastInsertId)"
	//ACCT_SPOT_HOLD_LTC="$(curl -s -d "apikey=$TmuApiKey&rarity=0&currency_id=$CurrencyLTC&title=OKEx Spot- Hold" $BwServerUrl/accounts | jq .LastInsertId)"

	// 3.3.5 We will need expense accounts for each currency for the variety of fees that we will encounter.
	//ACCT_FEE_BTC="$(curl -s -d "apikey=$TmuApiKey&rarity=0&currency_id=$CurrencyBTC&title=Fee" $BwServerUrl/accounts | jq .LastInsertId)"
	//ACCT_FEE_LTC="$(curl -s -d "apikey=$TmuApiKey&rarity=0&currency_id=$CurrencyLTC&title=Fee" $BwServerUrl/accounts | jq .LastInsertId)"
	fmt.Printf("Section 3.3 success.\n")

	// 3.4 Establish some necessary categories

	// 3.4.1 In order to produce Balance Sheet and Income Statement reports we must have these categories:
	//CAT_ASSETS="$(curl -s -d "apikey=$TmuApiKey&symbol=A&title=Assets" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_LIABILITIES="$(curl -s -d "apikey=$TmuApiKey&symbol=L&title=Liabilities" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_EQUITY="$(curl -s -d "apikey=$TmuApiKey&symbol=Eq&title=Equity" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_REVENUE="$(curl -s -d "apikey=$TmuApiKey&symbol=R&title=Revenue" $BwServerUrl/categories | jq .LastInsertId)"
	//CAT_EXPENSES="$(curl -s -d "apikey=$TmuApiKey&symbol=Ex&title=Expenses" $BwServerUrl/categories | jq .LastInsertId)"

	// 3.4.2 We will need a general ability to find all funding, spot-available, and spot-hold accounts so we need these categories.
	CatFunding = PostBwLid(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=F&title=Funding", TmuApiKey))

	CatSpotAvailable = PostBwLid(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=SA&title=Spot available", TmuApiKey))

	CatSpotHold = PostBwLid(httpClient, fmt.Sprintf(
		"%s/categories", BwServerUrl), fmt.Sprintf("apikey=%s&symbol=SH&title=Spot hold", TmuApiKey))

	// 3.5 Now tag these accounts with suitable categories.  Just do it, we don't care about saving any return values.
	//curl -s -d "apikey=$TmuApiKey&account_id=$AcctLocalWalletBTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_LCL_WALLET_LTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$AcctFundingBTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_FUNDING_LTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_SPOT_AVAIL_BTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_SPOT_AVAIL_LTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_SPOT_HOLD_BTC&category_id=$CAT_ASSETS" $BwServerUrl/acctcats

	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_FEE_BTC&category_id=$CAT_EXPENSES" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_FEE_LTC&category_id=$CAT_EXPENSES" $BwServerUrl/acctcats

	//curl -s -d "apikey=$TmuApiKey&account_id=$AcctEquity&category_id=$CAT_EQUITY" $BwServerUrl/acctcats

	//curl -s -d "apikey=$TmuApiKey&account_id=$AcctFundingBTC&category_id=$CatFunding" $BwServerUrl/acctcats
	_ = PostBwLid(httpClient, fmt.Sprintf(
		"%s/acctcats", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&category_id=%d", TmuApiKey, AcctFundingBTC, CatFunding))
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_FUNDING_LTC&category_id=$CatFunding" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_SPOT_AVAIL_BTC&category_id=$CatSpotAvailable" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_SPOT_AVAIL_LTC&category_id=$CatSpotAvailable" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_SPOT_HOLD_BTC&category_id=$CatSpotHold" $BwServerUrl/acctcats
	//curl -s -d "apikey=$TmuApiKey&account_id=$ACCT_SPOT_HOLD_LTC&category_id=$CatSpotHold" $BwServerUrl/acctcats

	// 3.6 Get read, read-trade, and read-withdraw credentials from the OKCatbox for this user.  As with the real OKEx API we'll need access credentials.  This OKCatbox endpoint is a convenience to make it easy to get credentials.  The real OKEx server doesn't issue credentials via the API.
	UserID := "moe"

	// 3.6.1 read
	OkCatboxCredentialsFileRead := "okcatbox-read.json"
	cbCredentialsRead := buildOKCatboxCredentials(httpClient, CatboxURL, CredentialsRequestBody{UserID: UserID, Type: "read"}, OkCatboxCredentialsFileRead)

	// 3.6.2 read-trade
	OkCatboxCredentialsFileReadTrade := "okcatbox-read-trade.json"
	_ = buildOKCatboxCredentials(httpClient, CatboxURL, CredentialsRequestBody{UserID: UserID, Type: "read"}, OkCatboxCredentialsFileReadTrade)

	// 3.6.3 read-withdrawal
	OkCatboxCredentialsFileReadWithdraw := "okcatbox-read-withdraw.json"
	_ = buildOKCatboxCredentials(httpClient, CatboxURL, CredentialsRequestBody{UserID: UserID, Type: "read"}, OkCatboxCredentialsFileReadWithdraw)

	// parse this into json so we can access it later

	//OkCatboxCredentialsFileReadTrade=okcatbox-read-trade.json
	//curl -s -X POST $CatboxURL/catbox/credentials --data "{\"UserID\":\"$UserID\",\"type\":\"read-trade\"}" --output $OkCatboxCredentialsFileReadTrade

	fmt.Printf("Section 3 success.  I have established the test monkey user.\n\n")

	// 4. Setup okconnect.

	// 4.1 Build the configuration file
	okconnectCfg := config.Config{
		BookwerxConfig: config.BookwerxConfig{
			APIKey:           TmuApiKey,
			BaseURL:          BwServerUrl,
			CatDeposit:       CatDeposit,
			CatFunding:       CatFunding,
			CatSpotAvailable: CatSpotAvailable,
			CatSpotHold:      CatSpotAvailable,
		},
		OKExConfig: config.OKExConfig{
			Credentials: OkCatboxCredentialsFileRead,
			BaseURL:     CatboxURL,
		},
	}

	out, err = yaml.Marshal(okconnectCfg)
	if err != nil {
		fmt.Printf("Error marshalling the okconnect config: err=%v\n", err)
		os.Exit(1)
	}
	err = ioutil.WriteFile("okconnect.yaml", out, 0600)
	if err != nil {
		fmt.Printf("Error writing the okconnect config to okconnect.yaml: err=%v\n", err)
		os.Exit(1)
	}
	//fmt.Printf("Test monkey's bookwerx config=\n%#v\n", okconnectCfg)
	okconnectConfigS, _ := json.MarshalIndent(okconnectCfg, "", "  ")
	fmt.Printf("okconnect config=\n%s\n\n", string(okconnectConfigS))

	fmt.Printf("Section 4 success.  I have configured okconnect.\n\n")

	// 5. Initial equity for the TMU
	TXID := PostBwLid(httpClient, fmt.Sprintf(
		"%s/transactions", BwServerUrl), fmt.Sprintf("apikey=%s&notes=Initial Equity&time=2020-05-01T12:34:55.000Z", TmuApiKey))
	_ = PostBwLid(httpClient, fmt.Sprintf(
		"%s/distributions", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&amount=2&amount_exp=0&transaction_id=%d", TmuApiKey, AcctLocalWalletBTC, TXID))
	_ = PostBwLid(httpClient, fmt.Sprintf(
		"%s/distributions", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&amount=-2&amount_exp=0&transaction_id=%d", TmuApiKey, AcctEquity, TXID))

	fmt.Printf("Section 5 success.  I have created the initial equity transaction for the TMU.\n\n")

	// 6. Simulate the deposit of BTC into the funding account.  This is a tedious and difficult issue for a variety of reasons.  Therefore, at this point, we will use this convenience endpoint from the OKCatbox where we can easily assert a deposit. This OKCatbox endpoint is a convenience to make it easy to make deposits.  The real OKEx server doesn't manage deposits via the API.

	// 6.1 Make the deposit manually to the catbox. We use the cbCredentialsRead merely to identify the user.
	//_ = PostCatboxDeposit(httpClient, CatboxURL, fmt.Sprintf("&apikey=%s&currency_symbol=BTC&quan=1.5", cbCredentialsRead.Key))
	//_ = PostCatboxDeposit(httpClient, CatboxURL, DepositRequestBody{}, cbCredentialsRead.Key))
	_ = PostCatboxDeposit(httpClient, CatboxURL, DepositRequestBody{
		Apikey:         cbCredentialsRead.Key,
		CurrencySymbol: "BTC",
		Quan:           "1.5",
		Time:           "2021",
	})
	fmt.Printf("Section 6.1 success.\n")

	// 6.2 Let's use okconnect to compare the user's balances in Bookwerx with the corresponding balances in the OKCatbox.  We should detect a discrepancy because the OKCatbox has a deposit,  but we haven't yet made a matching transaction on the user's books.
	out1, err := exec.Command("okconnect", "compare", "-config", "okconnect.yaml").Output()
	if err != nil {
		fmt.Printf("Cannot execute okconnect 6.2: err=%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("okconnect output 6.2=%s\n", out1)
	comparison := make([]compare.Comparison, 0)
	dec := json.NewDecoder(bytes.NewReader(out1))
	dec.DisallowUnknownFields()
	err = dec.Decode(&comparison)
	if err != nil {
		fmt.Printf("Cannot decode okconnect result 6.2: %v\n", err)
		os.Exit(1)
	}

	if len(comparison) != 1 {
		fmt.Printf("okconnect should see only 1 discrepency.  Instead it sees %d\n", len(comparison))
	}
	fmt.Printf("Section 6.2 success.\n")

	// 6.3 Now create the bookwerx transaction on our user's books.
	TXID = PostBwLid(httpClient, fmt.Sprintf(
		"%s/transactions", BwServerUrl), fmt.Sprintf("apikey=%s&notes=Xfer BTC to OKEx&time=2020-05-01T12:34:55.000Z", TmuApiKey))
	_ = PostBwLid(httpClient, fmt.Sprintf(
		"%s/distributions", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&amount=15&amount_exp=-1&transaction_id=%d", TmuApiKey, AcctFundingBTC, TXID))
	_ = PostBwLid(httpClient, fmt.Sprintf(
		"%s/distributions", BwServerUrl), fmt.Sprintf("apikey=%s&account_id=%d&amount=-15&amount_exp=-1&transaction_id=%d", TmuApiKey, AcctLocalWalletBTC, TXID))
	fmt.Printf("Section 6.3 success. I have created the bookwerx deposit tx on the TMU user's books\n")

	// 6.4 Let's use okconnect again to compare the user's balances in Bookwerx with the corresponding balances in the OKCatbox. Now there should be zero discrepancies.
	out1, err = exec.Command("okconnect", "compare", "-config", "okconnect.yaml").Output()
	if err != nil {
		fmt.Printf("Cannot execute okconnect 6.3: err=%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("okconnect output 6.3=%s\n", out1)
	comparison = make([]compare.Comparison, 0)
	dec = json.NewDecoder(bytes.NewReader(out1))
	dec.DisallowUnknownFields()
	err = dec.Decode(&comparison)
	if err != nil {
		fmt.Printf("Cannot decode okconnect result 6.3: %v\n", err)
		os.Exit(1)
	}

	if len(comparison) != 0 {
		fmt.Printf("okconnect not see any discrepencies.  Instead it sees %d\n", len(comparison))
	}
	fmt.Printf("Section 6.4 success.\n")

	fmt.Printf("Section 6. success. I have transferred coin from the TMU's local wallet into a catbox funding account.\n")

	// 7. Things are going to start happening now!  The next step is to transfer some BTC from the funding account (6) into the spot market (1).  This is something that okconnect can easily do.

	//okconnect transfer -currency BTC -quan 1.25 -from 6 -to 1 -config okconnect.yaml

	// 8. Finally, let's run some tests of okprobe
	testOKProbe(CatboxURL, "accountCurrencies", "", OkCatboxCredentialsFileRead, OkCatboxCredentialsFileReadTrade, OkCatboxCredentialsFileReadWithdraw)
	testOKProbe(CatboxURL, "accountDepositAddress", "?currency=BTC", OkCatboxCredentialsFileRead, OkCatboxCredentialsFileReadTrade, OkCatboxCredentialsFileReadWithdraw)
	testOKProbe(CatboxURL, "accountDepositHistory", "", OkCatboxCredentialsFileRead, OkCatboxCredentialsFileReadTrade, OkCatboxCredentialsFileReadWithdraw)
	testOKProbe(CatboxURL, "accountDepositHistoryByCur", "", OkCatboxCredentialsFileRead, OkCatboxCredentialsFileReadTrade, OkCatboxCredentialsFileReadWithdraw)
	//testOKProbe(CatboxURL, "accountLedger", "", OkCatboxCredentialsFileRead, OkCatboxCredentialsFileReadTrade, OkCatboxCredentialsFileReadWithdraw)
	//testOKProbe(CatboxURL, "accountTransfer", "", OkCatboxCredentialsFileRead, OkCatboxCredentialsFileReadTrade, OkCatboxCredentialsFileReadWithdraw)
	testOKProbe(CatboxURL, "accountWallet", "", OkCatboxCredentialsFileRead, OkCatboxCredentialsFileReadTrade, OkCatboxCredentialsFileReadWithdraw)
	//testOKProbe(CatboxURL, "accountWithdrawal", "", OkCatboxCredentialsFileRead, OkCatboxCredentialsFileReadTrade, OkCatboxCredentialsFileReadWithdraw)
	testOKProbe(CatboxURL, "accountWithdrawalFee", "", OkCatboxCredentialsFileRead, OkCatboxCredentialsFileReadTrade, OkCatboxCredentialsFileReadWithdraw)
	testOKProbe(CatboxURL, "spotAccounts", "", OkCatboxCredentialsFileRead, OkCatboxCredentialsFileReadTrade, OkCatboxCredentialsFileReadWithdraw)
}

func POST(client *httpclient.Client, url string, body io.Reader, headers http.Header) []byte {

	resp, err := client.Post(url, body, headers)
	if err != nil {
		fmt.Printf("Cannot POST to %s: %v\n", url, err)
		os.Exit(1)
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading from POST response: URL=%s, err=%v\n", url, err)
		os.Exit(1)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("Status code error: Expected status=200, Received=%d, URL=%s\nbody=%s\n", resp.StatusCode, url, string(responseBody))
		os.Exit(1)
	}

	return responseBody
}

// When the OKCatbox executes it needs some configuration.
// Duplicated from github.com/bostontrader/okcatbox.  Factor this out.
type Config struct {
	Bookwerx   Bookwerx
	ListenAddr string
}

// The OKCatbox will use a Bookwerx server for its internal operation.
// Duplicated from github.com/bostontrader/okcatbox.  Factor this out.
type Bookwerx struct {
	APIKey string
	Server string

	// Transactions may be tagged as deposits.
	CatDeposit uint32 `yaml:"cat_deposit"`

	// Customer accounts shall be tagged with these categories where applicable.
	// These are duplicated with TransferCats.  Figure this out.
	CatFunding       uint32 `yaml:"cat_funding"`
	CatSpotAvailable uint32 `yaml:"cat_spot_available"`
	CatSpotHold      uint32 `yaml:"cat_spot_hold"`

	// Any hot wallet shall be tagged with this category.
	CatHotWallet uint32 `yaml:"hot_wallet_cat"`

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
	UserID string
	Type   string
}

func PostBWCredentials(httpClient *httpclient.Client, baseURL string) string {

	url := fmt.Sprintf("%s/apikeys", baseURL)
	responseBody := POST(httpClient, url, nil, nil)

	type BwApiKey struct {
		APIKEY string `json:"apikey"`
	}

	var BookwerxCBAPIKey BwApiKey
	dec := json.NewDecoder(bytes.NewReader(responseBody))
	err := dec.Decode(&BookwerxCBAPIKey)
	if err != nil {
		fmt.Printf("JSON Decode error: Body=%s, err=%v\n", string(responseBody), err)
		os.Exit(1)
	}

	return BookwerxCBAPIKey.APIKEY
}

func PostCatboxCredentials(httpClient *httpclient.Client, baseURL string, credentialsRequestBody CredentialsRequestBody) utils.Credentials {

	url := fmt.Sprintf("%s/catbox/credentials", baseURL)
	h := make(map[string][]string)
	h["Content-Type"] = []string{"application/json"}
	b, err := json.Marshal(credentialsRequestBody)
	if err != nil {
		fmt.Printf("JSON Encode error: Obj=%v, err=%v\n", credentialsRequestBody, err)
		os.Exit(1)
	}
	responseBody := POST(httpClient, url, bytes.NewReader(b), h)

	var credentials utils.Credentials
	dec := json.NewDecoder(bytes.NewReader(responseBody))
	err = dec.Decode(&credentials)
	if err != nil {
		fmt.Printf("JSON Decode error: Body=%s, err=%v\n", string(responseBody), err)
		os.Exit(1)
	}

	return credentials
}

// Duplicated from github.com/bostontrader/okcatbox.  Factor this out.
type DepositRequestBody struct {
	Apikey         string
	CurrencySymbol string
	Quan           string
	Time           string
}

func PostCatboxDeposit(httpClient *httpclient.Client, baseURL string, depositRequestBody DepositRequestBody) []byte {

	methodName := "oktest:main.go:PostCatboxDeposit"
	url := fmt.Sprintf("%s/catbox/deposit", baseURL)
	reqHeaders := make(map[string][]string)
	reqHeaders["Content-Type"] = []string{"application/json"}

	b, err := json.Marshal(depositRequestBody)
	if err != nil {
		fmt.Printf("%s: JSON Marshal error: Obj=%v, err=%v\n", methodName, depositRequestBody, err)
		os.Exit(1)
	}
	responseBody := POST(httpClient, url, bytes.NewReader(b), reqHeaders)

	//var credentials utils.Credentials
	//dec := json.NewDecoder(bytes.NewReader(responseBody))
	//err = dec.Decode(&credentials)
	//if err != nil {
	//fmt.Printf("JSON Decode error: Body=%s, err=%v\n", string(responseBody), err)
	//os.Exit(1)
	//}

	return responseBody
}

// Post to bookwerx and get a Last Insert ID back.
func PostBwLid(httpClient *httpclient.Client, url string, body string) uint32 {

	h := make(map[string][]string)
	h["Content-Type"] = []string{"application/x-www-form-urlencoded"}
	responseBody := POST(httpClient, url, strings.NewReader(body), h)

	var lid LID
	dec := json.NewDecoder(bytes.NewReader(responseBody))
	err := dec.Decode(&lid)
	if err != nil {
		fmt.Printf("JSON Decode error: Body=%s, err=%v\n", string(responseBody), err)
		os.Exit(1)
	}

	return lid.LastInsertID
}

func buildOKCatboxCredentials(httpClient *httpclient.Client, baseURL string, credentialsRequestBody CredentialsRequestBody, credentialsFileName string) utils.Credentials {

	methodName := "oktest:main.go:buildOKCatboxCredentials"
	cbc := PostCatboxCredentials(httpClient, baseURL, credentialsRequestBody)

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
