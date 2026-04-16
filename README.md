# Grimoire

> [!NOTE]
> **This project is in active development.** Things may change or break. For questions, bug reports, or feedback, [open an issue](https://github.com/leshicodes/grimoire/issues).

Grimoire is a self-hosted web app for checking a TCGPlayer seller's inventory against a card wantlist. Paste in a list of cards (or drop a Moxfield or Archidekt deck URL), point it at a seller, and it'll tell you what they have in stock.

The original use case was local game store discovery: here's my deck, which of these sellers actually has the cards I need?

Deck importing is powered by [Scrybrary](https://github.com/leshicodes/scrybrary), my companion CLI/library for parsing and fetching MTG decklists.

---

## Running it

```bash
docker compose up
```

Then visit `http://localhost:8081`.

To build and run locally without Docker:

```bash
make build
./grimoire
```

---

## Usage

1. Provide a seller inventory source. You can either:
   - Upload a TCGPlayer inventory CSV (recommended, download from the TCGPlayer Seller Portal under Inventory > Export All Inventory)
   - Or enter a TCGPlayer store URL (see the warning below)

2. Paste your card wantlist into the textarea, one card per line. Quantities are optional.

   ```
   Sol Ring
   1 Lightning Bolt
   3x Forest
   ```

3. Optionally import a wantlist directly from a Moxfield or Archidekt deck URL using the import section below the textarea.

4. Hit search. Results show which cards the seller has in stock and at what price.

> **TCGPlayer store URL scraping** may violate TCGPlayer's Terms of Service. Use it at your own risk. The CSV upload is the safer and more reliable path. But only Sellers can export their inventory, not buyer :/

---

## Configuration

| Variable    | Default | Description             |
|-------------|---------|-------------------------|
| `PORT`      | `8081`  | Port the server listens on |
| `LOG_LEVEL` | `info`  | `info` or `debug`       |

Both can be set in `docker-compose.yml` or as environment variables.

---

## Supported inventory sources

| Source                 | Status |
|------------------------|--------|
| TCGPlayer CSV upload   | yes    |
| TCGPlayer store URL    | partial (TOS risk) |

## Supported deck import sources

| Platform   | Import |
|------------|--------|
| Moxfield   | yes    |
| Archidekt  | yes    |

---

## For Store Owners

Grimoire is designed to be a tool your customers can use **at your store** to check their wantlist against your inventory in real time. Here's how to set it up in about 2 minutes.

### 1. Export your inventory from TCGPlayer

Log into the [TCGPlayer Seller Portal](https://store.tcgplayer.com/), go to **Inventory → Export All Inventory**, and download the CSV file.

### 2. Upload your inventory to Grimoire

Go to [/setup](/setup) (or `https://beta.grimoire.leshicodes.info/setup` on the hosted instance).

- Paste your **TCGPlayer store URL** (e.g. `https://www.tcgplayer.com/sellers/Your-Store/abc123`) — find it by visiting your store page on TCGPlayer and copying the URL from the address bar.
- Optionally enter a **display name** (how your store appears to customers).
- Upload the **inventory CSV** you exported in step 1.
- Hit **Upload Inventory**.

### 3. Get your permanent link and QR code

After uploading you'll see:

- A **permanent store URL** like `https://beta.grimoire.leshicodes.info/s/Your-Store/abc123`
- A **QR code** — right-click to save it, then print it and put it somewhere customers can see it (counter, table tents, etc.)

The URL and QR code **never change**. When your inventory updates, just re-upload the CSV — same link, fresh data.

### 4. Customers use it

Customers scan the QR code, paste their wantlist (or import a Moxfield/Archidekt deck), and hit Search. They see exactly which cards you have in stock, the condition, and the price.

They can also hit **Share results** to get a 48-hour link they can send to you directly — so you know exactly what to pull from the shelf.

> **Re-upload whenever your inventory changes.** The cached inventory persists until you update it. More frequent updates = more accurate results for customers.

---

## A note on how this was built

This project uses AI agent assistance. No, I am not vibecoding. When I need to brain dump something into documentation or knock out a straightforward function, I lean on an LLM rather than do it by hand. That said, every line of code here has been read, reasoned about, and signed off on by me.

I have about 10 years of industry experience, mostly on the DevOps and infrastructure side. Go is relatively new territory for me (~1 year in), so if you see something that could be done better, I genuinely want to hear it. Open an issue or start a discussion; constructive feedback is always welcome.
