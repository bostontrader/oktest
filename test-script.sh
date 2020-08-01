#!/bin/sh

go get github.com/bostontrader/okcatbox
go get github.com/bostontrader/okconnect
go get github.com/bostontrader/okprobe

go install github.com/bostontrader/okcatbox
go install github.com/bostontrader/okconnect
go install github.com/bostontrader/okprobe

# In this test scenario, we're going to figure out how to persuade OKConnect to place an order to sell 1 BTC and buy 25 LTC at the implied price of LTCBTC = 0.04 and then to make the sale.  We will work our way through the entire life-cycle of doing this including:
#  * The initial equity transaction on the user's books
#  * The deposit of BTC with the OKCatbox
#  * Transfer BTC from the OKCatbox funding account to the spot market
#  * Place an order to trade BTC for LTC
#  * Execute the order
#  * Transfer LTC from the OKCatbox spot market to the funding account
#  * Withdraw LTC from the OKCatbox.

# In order to do this we're going to have to manage two sets of bookkeeping records.  We will do so by using [a publicly available Bookwerx Core server](https://github.com/bostontrader/bookwerx-core-rust).  The first set of books is for the OKCatbox itself.  In order to do its thing it needs to know things such as customer balances, hence the need for bookkeeping.  The second set of books is for the user acting as a customer of the OKCatbox.

# Both sets of books should be constantly in sync.  We will use OKConnect to verify that said balances are in agreement as well as to take any action that affects both sets of books.

# That said...

# 1. Install, configure, and execute the OKCatbox
CATBOX_URL=http://localhost:8090

# 1.1 Using the demo Bookwerx server, get credentials for the OKCatbox and setup some demo data.  Recall that this is the bookkeeping configuration that the OKCatbox uses for its own personal consumption.

BW_SERVER_URL=http://185.183.96.73:3003
APIKEY="$(curl -X POST $BW_SERVER_URL/apikeys | jq -r .apikey)"

# 1.2 The OKCatbox will support these currencies...
CURRENCY_BTC="$(curl -d "apikey=$APIKEY&rarity=0&symbol=BTC&title=Bitcoin" $BW_SERVER_URL/currencies | jq .LastInsertId)"
CURRENCY_LTC="$(curl -d "apikey=$APIKEY&rarity=0&symbol=LTC&title=Litecoin" $BW_SERVER_URL/currencies | jq .LastInsertId)"

HOT_WALLET_BTC="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=Hot wallet" $BW_SERVER_URL/accounts | jq .LastInsertId)"
HOT_WALLET_LTC="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=Hot wallet" $BW_SERVER_URL/accounts | jq .LastInsertId)"

CAT_ASSETS="$(curl -d "apikey=$APIKEY&symbol=A&title=Assets" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_LIABILITIES="$(curl -d "apikey=$APIKEY&symbol=L&title=Liabilities" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_EQUITY="$(curl -d "apikey=$APIKEY&symbol=Eq&title=Equity" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_REVENUE="$(curl -d "apikey=$APIKEY&symbol=R&title=Revenue" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_EXPENSES="$(curl -d "apikey=$APIKEY&symbol=Ex&title=Expenses" $BW_SERVER_URL/categories | jq .LastInsertId)"

CAT_FUNDING="$(curl -d "apikey=$APIKEY&symbol=F&title=Funding" $BW_SERVER_URL/categories | jq .LastInsertId)"

# 1.5 Any hot wallet accounts shall be tagged with this category...
CAT_HOT="$(curl -d "apikey=$APIKEY&symbol=H&title=Hot wallet" $BW_SERVER_URL/categories | jq .LastInsertId)"

# ... thus
curl -d "apikey=$APIKEY&account_id=$HOT_WALLET_BTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$HOT_WALLET_LTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$HOT_WALLET_BTC&category_id=$CAT_HOT" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$HOT_WALLET_LTC&category_id=$CAT_HOT" $BW_SERVER_URL/acctcats

# 1.6 Build a config file for okcatbox
echo "bookwerx:" > okcatbox.yaml
echo "  apikey: $APIKEY" >> okcatbox.yaml
echo "  server: $BW_SERVER_URL" >> okcatbox.yaml
echo "  funding_cat: $CAT_FUNDING" >> okcatbox.yaml
echo "  hot_wallet_cat: $CAT_HOT" >> okcatbox.yaml
echo "listenaddr: :8090" >> okcatbox.yaml
cat okcatbox.yaml


# 1.7 Start the catbox daemonized
okcatbox -config=okcatbox.yaml &

# 2. Now setup the test monkey user.

# 2.1 In the beginning... The user has nothing.  He must first establish his own account with the Bookwerx Core server.
# Note: We are going to reuse variable names, but that's ok because we don't need the values from the OKCatbox installation any more.
APIKEY="$(curl -X POST $BW_SERVER_URL/apikeys | jq -r .apikey)"
echo "User's Bookwerx APIKEY=$APIKEY"

# 2.1.1 Since we are going to use BTC and LTC in our subsequent transactions, we must define them as currencies in Bookwerx. We have already done this for the OKCatbox books, but we are using the same currencies for the user's books and we must define them separately there.
CURRENCY_BTC="$(curl -d "apikey=$APIKEY&rarity=0&symbol=BTC&title=Bitcoin" $BW_SERVER_URL/currencies | jq .LastInsertId)"
CURRENCY_LTC="$(curl -d "apikey=$APIKEY&rarity=0&symbol=LTC&title=Litecoin" $BW_SERVER_URL/currencies | jq .LastInsertId)"

# 2.1.3 Establish some necessary bookkeeping accounts for the user.  Notice that several of the accounts have identical titles.  They are differentiated according to their currencies.

# 2.1.4 We must have owner's equity to get the party started.
ACCT_EQUITY="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=Owners Equity" $BW_SERVER_URL/accounts | jq .LastInsertId)"

# 2.1.5 We must have asset accounts for our local wallets.
ACCT_LCL_WALLET_BTC="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=Local Wallet" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_LCL_WALLET_LTC="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=Local Wallet" $BW_SERVER_URL/accounts | jq .LastInsertId)"

# 2.1.6 We must have asset accounts for our funding accounts on OKEx
ACCT_FUNDING_BTC="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=OKEx Funding" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_FUNDING_LTC="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=OKEx Funding" $BW_SERVER_URL/accounts | jq .LastInsertId)"

# 2.1.7 We must have asset accounts for our balances in the spot trading area of OKEx.  Not merely one, but two balances, available and amounts on hold.
ACCT_SPOT_BTC_AVAIL="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=OKEx Spot- Available" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_SPOT_LTC_AVAIL="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=OKEx Spot- Available" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_SPOT_BTC_HOLD="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=OKEx Spot- Hold" $BW_SERVER_URL/accounts | jq .LastInsertId)"
# ACCT_SPOT_LTC_HOLD="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=OKEx Spot- Hold" $BW_SERVER_URL/accounts | jq .LastInsertId)"

# 2.1.8 We will need expense accounts for each currency for the variety of fees that we will encounter.
ACCT_FEE_BTC="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=Fee" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_FEE_LTC="$(curl -d "apikey=$APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=Fee" $BW_SERVER_URL/accounts | jq .LastInsertId)"

# 2.1.9 In order to produce Balance Sheet and Income Statement reports we must have these categories:
CAT_ASSETS="$(curl -d "apikey=$APIKEY&symbol=A&title=Assets" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_LIABILITIES="$(curl -d "apikey=$APIKEY&symbol=L&title=Liabilities" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_EQUITY="$(curl -d "apikey=$APIKEY&symbol=Eq&title=Equity" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_REVENUE="$(curl -d "apikey=$APIKEY&symbol=R&title=Revenue" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_EXPENSES="$(curl -d "apikey=$APIKEY&symbol=Ex&title=Expenses" $BW_SERVER_URL/categories | jq .LastInsertId)"

# 2.1.10 We will need a general ability to find all funding accounts so we need this category.
CAT_FUNDING="$(curl -d "apikey=$APIKEY&symbol=F&title=Funding" $BW_SERVER_URL/categories | jq .LastInsertId)"

# 2.1.11 Now tag these accounts with suitable categories.  Just do it, we don't care about saving any return value.
curl -d "apikey=$APIKEY&account_id=$ACCT_LCL_WALLET_BTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$ACCT_LCL_WALLET_LTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$ACCT_FUNDING_BTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$ACCT_FUNDING_LTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$ACCT_SPOT_BTC_AVAIL&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$ACCT_SPOT_LTC_AVAIL&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$ACCT_SPOT_BTC_HOLD&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats

curl -d "apikey=$APIKEY&account_id=$ACCT_FEE_BTC&category_id=$CAT_EXPENSES" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$ACCT_FEE_LTC&category_id=$CAT_EXPENSES" $BW_SERVER_URL/acctcats

curl -d "apikey=$APIKEY&account_id=$ACCT_EQUITY&category_id=$CAT_EQUITY" $BW_SERVER_URL/acctcats

curl -d "apikey=$APIKEY&account_id=$ACCT_FUNDING_BTC&category_id=$CAT_FUNDING" $BW_SERVER_URL/acctcats
curl -d "apikey=$APIKEY&account_id=$ACCT_FUNDING_LTC&category_id=$CAT_FUNDING" $BW_SERVER_URL/acctcats

TXID="$(curl -d "apikey=$APIKEY&notes=Initial Equity&time=2020-05-01T12:34:55.000Z" $BW_SERVER_URL/transactions | jq .LastInsertId)"
curl -d "&account_id=$ACCT_LCL_WALLET_BTC&apikey=$APIKEY&amount=2&amount_exp=0&transaction_id=$TXID" $BW_SERVER_URL/distributions
curl -d "&account_id=$ACCT_EQUITY&apikey=$APIKEY&amount=-2&amount_exp=0&transaction_id=$TXID" $BW_SERVER_URL/distributions

# 2.2 Get credentials from the OKCatbox for this user.  As with the real OKEx API we'll need access credentials.  This OKCatbox endpoint is a convenience to make it easy to get credentials.  The real OKEx server doesn't issue credentials via the API.
OKCATBOX_CREDENTIALS_FILE=okcatbox.json
curl -X POST $CATBOX_URL/catbox/credentials --output $OKCATBOX_CREDENTIALS_FILE

# 2.3 Simulate the deposit of BTC into the funding account.  This is a tedious and difficult issue for a variety of reasons.  Therefore, at this point, we will...
#
# 2.3.1 ... use this convenience endpoint from the OKCatbox where we can easily assert a deposit. This OKCatbox endpoint is a convenience to make it easy to make deposits.  The real OKEx server doesn't manage deposits via the API.
# Make sure these params jive with those in the next section.
curl -d "&apikey=$(cat $OKCATBOX_CREDENTIALS_FILE | jq -r .api_key)&currency_symbol=BTC&quan=1.5&time=2020-07-21T" $CATBOX_URL/catbox/deposit

# 2.3.2 ... and then create the bookwerx transaction on our user's books.
# Make sure these params jive with those in the prior section.
TXID="$(curl -d "apikey=$APIKEY&notes=Xfer BTC to OKEx&time=2020-05-01T12:34:55.000Z" $BW_SERVER_URL/transactions | jq .LastInsertId)"
curl -d "&account_id=$ACCT_FUNDING_BTC&apikey=$APIKEY&amount=15&amount_exp=-1&transaction_id=$TXID" $BW_SERVER_URL/distributions
curl -d "&account_id=$ACCT_LCL_WALLET_BTC&apikey=$APIKEY&amount=-15&amount_exp=-1&transaction_id=$TXID" $BW_SERVER_URL/distributions

# 3. Setup okconnect.
echo "bookwerxconfig:" > okconnect.yaml
echo "  apikey: $APIKEY" >> okconnect.yaml
echo "  server: $BW_SERVER_URL" >> okconnect.yaml
echo "  funding_cat: $CAT_FUNDING" >> okconnect.yaml  # This is supposed to be a category in the user's books!
echo "okexconfig:" >> okconnect.yaml
echo "  credentials: $OKCATBOX_CREDENTIALS_FILE" >> okconnect.yaml
echo "  server: $CATBOX_URL" >> okconnect.yaml
cat okconnect.yaml

# 4. Let's use okconnect to compare the user's balances in Bookwerx with the corresponding balances in the OKCatbox.
okconnect compare -config okconnect.yaml