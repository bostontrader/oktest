#!/bin/sh -x

go get github.com/bostontrader/okcatbox
go get github.com/bostontrader/okconnect
go get github.com/bostontrader/okprobe

go install github.com/bostontrader/okcatbox
go install github.com/bostontrader/okconnect
go install github.com/bostontrader/okprobe

# In this test scenario, we're going to figure out how to persuade OKConnect to place an order to sell 1 BTC and buy 25 LTC at the implied price of LTCBTC = 0.04 and then to make the sale.  We will then withdraw LTC from the exchange.  We will work our way through the entire life-cycle of doing this including:
#  * The initial equity transaction to record the user's BTC asset on the user's books
#  * The deposit of BTC with the OKCatbox
#  * Transfer BTC from the OKCatbox funding account to the spot market
#  * Place an order to trade BTC for LTC
#  * Execute the order
#  * Transfer LTC from the OKCatbox spot market to the funding account
#  * Withdraw LTC from the OKCatbox.

# In order to do this we're going to have to manage two sets of bookkeeping records.  We will do so by using [a publicly available Bookwerx Core server](https://github.com/bostontrader/bookwerx-core-rust).  The first set of books is for the OKCatbox itself.  In order to do its thing it needs to know things such as customer balances, hence the need for bookkeeping.  The second set of books is for the user acting as a customer of the OKCatbox.

# Both sets of books should be constantly in sync.  We will use OKConnect to verify that said balances are in agreement as well as to take any action that affects both sets of books.

# That said...

echo
echo "1. Install, configure, and execute the OKCatbox"
CATBOX_URL=http://localhost:8090

echo
echo "1.1 Using the demo Bookwerx server, get credentials for the OKCatbox and setup some demo data.  Recall that this is the bookkeeping configuration that the OKCatbox uses for its own personal consumption."

BW_SERVER_URL=http://185.183.96.73:3003

BW_CB_APIKEY="$(curl -s -X POST $BW_SERVER_URL/apikeys | jq -r .apikey)"

echo
echo "1.2 The OKCatbox will support these currencies..."
CURRENCY_BTC="$(curl -s -d "apikey=$BW_CB_APIKEY&rarity=0&symbol=BTC&title=Bitcoin" $BW_SERVER_URL/currencies | jq .LastInsertId)"
CURRENCY_LTC="$(curl -s -d "apikey=$BW_CB_APIKEY&rarity=0&symbol=LTC&title=Litecoin" $BW_SERVER_URL/currencies | jq .LastInsertId)"

echo
echo "1.3 The OKCatbox will need a hot wallet asset account for each of the supported currencies."
HOT_WALLET_BTC="$(curl -s -d "apikey=$BW_CB_APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=Hot wallet" $BW_SERVER_URL/accounts | jq .LastInsertId)"
HOT_WALLET_LTC="$(curl -s -d "apikey=$BW_CB_APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=Hot wallet" $BW_SERVER_URL/accounts | jq .LastInsertId)"

echo
echo "1.4 The OKCatbox will need these customary categories in order to produce balance sheets and income statements."
CAT_ASSETS="$(curl -s -d "apikey=$BW_CB_APIKEY&symbol=A&title=Assets" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_LIABILITIES="$(curl -s -d "apikey=$BW_CB_APIKEY&symbol=L&title=Liabilities" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_EQUITY="$(curl -s -d "apikey=$BW_CB_APIKEY&symbol=Eq&title=Equity" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_REVENUE="$(curl -s -d "apikey=$BW_CB_APIKEY&symbol=R&title=Revenue" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_EXPENSES="$(curl -s -d "apikey=$BW_CB_APIKEY&symbol=Ex&title=Expenses" $BW_SERVER_URL/categories | jq .LastInsertId)"

echo
echo "1.4.1 It will also need to tag customer accounts for funding, spot available, and spot hold. Said accounts will be created by the OKCatbox later, when required.  But we want the categories defined now."
CAT_FUNDING="$(curl -s -d "apikey=$BW_CB_APIKEY&symbol=F&title=Funding" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_SPOT_AVAILABLE="$(curl -s -d "apikey=$BW_CB_APIKEY&symbol=SA&title=Spot available" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_SPOT_HOLD="$(curl -s -d "apikey=$BW_CB_APIKEY&symbol=SH&title=Spot hold" $BW_SERVER_URL/categories | jq .LastInsertId)"

echo
echo "1.4.2 Any hot wallet accounts shall be tagged with this category..."
CAT_HOT_WALLET="$(curl -s -d "apikey=$BW_CB_APIKEY&symbol=H&title=Hot wallet" $BW_SERVER_URL/categories | jq .LastInsertId)"

# Tag each hot wallet account as an Asset and a Hot Wallet.
curl -s -d "apikey=$BW_CB_APIKEY&account_id=$HOT_WALLET_BTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$BW_CB_APIKEY&account_id=$HOT_WALLET_LTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$BW_CB_APIKEY&account_id=$HOT_WALLET_BTC&category_id=$CAT_HOT_WALLET" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$BW_CB_APIKEY&account_id=$HOT_WALLET_LTC&category_id=$CAT_HOT_WALLET" $BW_SERVER_URL/acctcats

echo
echo "1.5 Build a config file for okcatbox"
echo "bookwerx:" > okcatbox.yaml
echo "  apikey: $BW_CB_APIKEY" >> okcatbox.yaml
echo "  server: $BW_SERVER_URL" >> okcatbox.yaml
echo "  funding_cat: $CAT_FUNDING" >> okcatbox.yaml
echo "  spot_available_cat: $CAT_SPOT_AVAILABLE" >> okcatbox.yaml
echo "  spot_hold_cat: $CAT_SPOT_HOLD" >> okcatbox.yaml
echo "  hot_wallet_cat: $CAT_HOT_WALLET" >> okcatbox.yaml
echo "  transfer_cats:" >> okcatbox.yaml
echo "    1:" >> okcatbox.yaml
echo "      available: $CAT_SPOT_AVAILABLE" >> okcatbox.yaml
echo "      hold: $CAT_SPOT_HOLD" >> okcatbox.yaml
echo "    6:" >> okcatbox.yaml
echo "      available: $CAT_FUNDING" >> okcatbox.yaml
echo "      hold: -1" >> okcatbox.yaml
echo "listenaddr: :8090" >> okcatbox.yaml
cat okcatbox.yaml

echo
echo "1.6 Start the catbox daemonized"
okcatbox -config=okcatbox.yaml &

echo
echo "2. Now setup the test monkey user."

echo
echo "2.1 In the beginning... The user has nothing.  He must first establish his own account with the Bookwerx Core server.
# Note: We are going to reuse variable names, but that's ok because we don't need the values from the OKCatbox installation any more."
TMU_APIKEY="$(curl -s -X POST $BW_SERVER_URL/apikeys | jq -r .apikey)"

echo
echo "2.2 Since we are going to use BTC and LTC in our subsequent transactions, we must define them as currencies in Bookwerx. We have already done this for the OKCatbox books, but we are using the same currencies for the user's books and we must define them separately there."
CURRENCY_BTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&symbol=BTC&title=Bitcoin" $BW_SERVER_URL/currencies | jq .LastInsertId)"
CURRENCY_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&symbol=LTC&title=Litecoin" $BW_SERVER_URL/currencies | jq .LastInsertId)"

echo
echo "2.3 Establish some necessary bookkeeping accounts for the user.  Notice that several of the accounts have identical titles.  They are differentiated according to their currencies."

# 2.3.1 We must have owner's equity to get the party started.
ACCT_EQUITY="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=Owners Equity" $BW_SERVER_URL/accounts | jq .LastInsertId)"

# 2.3.2 We must have asset accounts for our local wallets.
ACCT_LCL_WALLET_BTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=Local Wallet" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_LCL_WALLET_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=Local Wallet" $BW_SERVER_URL/accounts | jq .LastInsertId)"

# 2.3.3 We must have asset accounts for our funding accounts on OKEx
ACCT_FUNDING_BTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=OKEx Funding" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_FUNDING_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=OKEx Funding" $BW_SERVER_URL/accounts | jq .LastInsertId)"

# 2.3.4 We must have asset accounts for our balances in the spot trading area of OKEx.  Not merely one, but two balances, available and amounts on hold.
ACCT_SPOT_AVAIL_BTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=OKEx Spot- Available" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_SPOT_AVAIL_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=OKEx Spot- Available" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_SPOT_HOLD_BTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=OKEx Spot- Hold" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_SPOT_HOLD_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=OKEx Spot- Hold" $BW_SERVER_URL/accounts | jq .LastInsertId)"

# 2.3.5 We will need expense accounts for each currency for the variety of fees that we will encounter.
ACCT_FEE_BTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_BTC&title=Fee" $BW_SERVER_URL/accounts | jq .LastInsertId)"
ACCT_FEE_LTC="$(curl -s -d "apikey=$TMU_APIKEY&rarity=0&currency_id=$CURRENCY_LTC&title=Fee" $BW_SERVER_URL/accounts | jq .LastInsertId)"

echo
echo "2.4 Establish some necessary categories"

# 2.4.1 In order to produce Balance Sheet and Income Statement reports we must have these categories:
CAT_ASSETS="$(curl -s -d "apikey=$TMU_APIKEY&symbol=A&title=Assets" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_LIABILITIES="$(curl -s -d "apikey=$TMU_APIKEY&symbol=L&title=Liabilities" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_EQUITY="$(curl -s -d "apikey=$TMU_APIKEY&symbol=Eq&title=Equity" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_REVENUE="$(curl -s -d "apikey=$TMU_APIKEY&symbol=R&title=Revenue" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_EXPENSES="$(curl -s -d "apikey=$TMU_APIKEY&symbol=Ex&title=Expenses" $BW_SERVER_URL/categories | jq .LastInsertId)"

# 2.4.2 We will need a general ability to find all funding, spot-available, and spot-hold accounts so we need these categories.
CAT_FUNDING="$(curl -s -d "apikey=$TMU_APIKEY&symbol=F&title=Funding" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_SPOT_AVAILABLE="$(curl -s -d "apikey=$TMU_APIKEY&symbol=SA&title=Spot available" $BW_SERVER_URL/categories | jq .LastInsertId)"
CAT_SPOT_HOLD="$(curl -s -d "apikey=$TMU_APIKEY&symbol=SH&title=Spot hold" $BW_SERVER_URL/categories | jq .LastInsertId)"

echo
echo "2.5 Now tag these accounts with suitable categories.  Just do it, we don't care about saving any return values."
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_LCL_WALLET_BTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_LCL_WALLET_LTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FUNDING_BTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FUNDING_LTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_AVAIL_BTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_AVAIL_LTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_HOLD_BTC&category_id=$CAT_ASSETS" $BW_SERVER_URL/acctcats

curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FEE_BTC&category_id=$CAT_EXPENSES" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FEE_LTC&category_id=$CAT_EXPENSES" $BW_SERVER_URL/acctcats

curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_EQUITY&category_id=$CAT_EQUITY" $BW_SERVER_URL/acctcats

curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FUNDING_BTC&category_id=$CAT_FUNDING" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_FUNDING_LTC&category_id=$CAT_FUNDING" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_AVAIL_BTC&category_id=$CAT_SPOT_AVAILABLE" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_AVAIL_LTC&category_id=$CAT_SPOT_AVAILABLE" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_HOLD_BTC&category_id=$CAT_SPOT_HOLD" $BW_SERVER_URL/acctcats
curl -s -d "apikey=$TMU_APIKEY&account_id=$ACCT_SPOT_HOLD_LTC&category_id=$CAT_SPOT_HOLD" $BW_SERVER_URL/acctcats

echo
echo "2.6 Get read and read-trade credentials from the OKCatbox for this user.  As with the real OKEx API we'll need access credentials.  This OKCatbox endpoint is a convenience to make it easy to get credentials.  The real OKEx server doesn't issue credentials via the API."
USER_ID=moe

OKCATBOX_CREDENTIALS_FILE_READ=okcatbox-read.json
curl -s -X POST $CATBOX_URL/catbox/credentials --data "{\"user_id\":\"$USER_ID\",\"type\":\"read\"}" --output $OKCATBOX_CREDENTIALS_FILE_READ

OKCATBOX_CREDENTIALS_FILE_READ_TRADE=okcatbox-read-trade.json
curl -s -X POST $CATBOX_URL/catbox/credentials --data "{\"user_id\":\"$USER_ID\",\"type\":\"read-trade\"}" --output $OKCATBOX_CREDENTIALS_FILE_READ_TRADE

echo
echo "3. Setup okconnect."
echo "bookwerxconfig:" > okconnect.yaml
echo "  apikey: $TMU_APIKEY" >> okconnect.yaml
echo "  server: $BW_SERVER_URL" >> okconnect.yaml
echo "  funding_cat: $CAT_FUNDING" >> okconnect.yaml  # This is supposed to be a category in the user's books!
echo "  spot_available_cat: $CAT_SPOT_AVAILABLE" >> okconnect.yaml
echo "  spot_hold_cat: $CAT_SPOT_HOLD" >> okconnect.yaml
echo "okexconfig:" >> okconnect.yaml
echo "  credentials: $OKCATBOX_CREDENTIALS_FILE_READ" >> okconnect.yaml
echo "  server: $CATBOX_URL" >> okconnect.yaml
cat okconnect.yaml

echo
echo "4. Start making some transactions."

echo
echo "4.1 Initial equity"
TXID="$(curl -s -d "apikey=$TMU_APIKEY&notes=Initial Equity&time=2020-05-01T12:34:55.000Z" $BW_SERVER_URL/transactions | jq .LastInsertId)"
curl -s -d "&account_id=$ACCT_LCL_WALLET_BTC&apikey=$TMU_APIKEY&amount=2&amount_exp=0&transaction_id=$TXID" $BW_SERVER_URL/distributions
curl -s -d "&account_id=$ACCT_EQUITY&apikey=$TMU_APIKEY&amount=-2&amount_exp=0&transaction_id=$TXID" $BW_SERVER_URL/distributions

echo
echo "4.2 Simulate the deposit of BTC into the funding account.  This is a tedious and difficult issue for a variety of reasons.  Therefore, at this point, we will..."

echo
echo "4.2.1 ... use this convenience endpoint from the OKCatbox where we can easily assert a deposit. This OKCatbox endpoint is a convenience to make it easy to make deposits.  The real OKEx server doesn't manage deposits via the API."

# Make sure these params jive with those in section 4.2.3.
curl -s -d "&apikey=$(cat $OKCATBOX_CREDENTIALS_FILE_READ | jq -r .api_key)&currency_symbol=BTC&quan=1.5&time=2020-07-21T" $CATBOX_URL/catbox/deposit

echo
echo "4.2.2. Let's use okconnect to compare the user's balances in Bookwerx with the corresponding balances in the OKCatbox.  We should detect a discrepancy because the OKCatbox has a deposit,  but we haven't yet made a matching transaction on the user's books."
okconnect compare -config okconnect.yaml > okconnect.out
if [ $(jq -r .[0].Category okconnect.out) != "F" ]; then echo "okconnect error"; exit 1; fi
if [ $(jq -r .[0].OKExBalance.Balance okconnect.out) != "1.5" ]; then echo "okconnect error"; exit 1; fi
if [ $(jq -r .[0].BookwerxBalance.Nil okconnect.out) != "true" ]; then echo "okconnect error"; exit 1; fi

echo
echo "4.2.3 Now create the bookwerx transaction on our user's books."
# Make sure these params jive with those in section 4.2.1.
TXID="$(curl -s -d "apikey=$TMU_APIKEY&notes=Xfer BTC to OKEx&time=2020-05-01T12:34:55.000Z" $BW_SERVER_URL/transactions | jq .LastInsertId)"
curl -s -d "&account_id=$ACCT_FUNDING_BTC&apikey=$TMU_APIKEY&amount=15&amount_exp=-1&transaction_id=$TXID" $BW_SERVER_URL/distributions
curl -s -d "&account_id=$ACCT_LCL_WALLET_BTC&apikey=$TMU_APIKEY&amount=-15&amount_exp=-1&transaction_id=$TXID" $BW_SERVER_URL/distributions

echo
echo "4.2.4. Let's use okconnect again to compare the user's balances in Bookwerx with the corresponding balances in the OKCatbox. Now there should be zero discrepancies."
okconnect compare -config okconnect.yaml > okconnect.out
if [ $(jq -r .[0] okconnect.out) != "null" ]; then echo "okconnect error"; exit 1; fi

echo
echo "5. Things are going to start happening now!  The next step is to transfer some BTC from the funding account (6) into the spot market (1).  This is something that okconnect can easily do."

#okconnect transfer -currency BTC -quan 1.25 -from 6 -to 1 -config okconnect.yaml