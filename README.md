# ⚡ FlashySpeed

**Your own personal file server — runs on your laptop or home server, no cloud required.**

Upload, browse, share, and stream your files from any device on your network. Everything runs as a single program with no complicated setup.

---

## What can it do?

| Feature | Description |
|---------|-------------|
| 📁 **File browser** | Browse folders, upload files, create folders, rename, and delete |
| 🔍 **Search** | Find any file instantly by name |
| 🔗 **Share links** | Create a public link for any file — anyone can download it, no login needed |
| ▶ **Media preview** | View images and play videos/audio directly in your browser |
| ☑ **Bulk actions** | Select multiple files to delete or download them all as a ZIP |
| 👥 **Multiple users** | Create accounts for family or team members with optional storage limits |
| ⚙ **Admin panel** | Manage users, see storage usage, and connect via Tailscale |
| 🔒 **Secure** | Password-protected accounts, HTTPS by default |

---

## Before you begin

You need two free programs installed on your computer:

### 1. Go (the programming language FlashySpeed is built with)
- Go to **https://go.dev/dl/**
- Download and run the installer for your system (Windows `.msi` or Linux)
- To check it worked, open a terminal and type: `go version`

### 2. Node.js (used to build the web interface)
- Go to **https://nodejs.org**
- Download the **LTS** version and run the installer
- To check it worked, open a terminal and type: `node --version`

### 3. Get the code
You also need Git installed to download the project:
- **Windows:** https://git-scm.com/download/win
- **Linux:** `sudo apt install git` (Ubuntu/Debian) or `sudo dnf install git` (Fedora)

---

## 🖥️ Running on Windows (local laptop)

### Step 1 — Download the project

Open **PowerShell** (search for it in the Start menu) and run:

```powershell
git clone https://github.com/Shaf2665/FlashSpeed.git
cd FlashSpeed
```

### Step 2 — Build the web interface

```powershell
cd web
npm install
npm run build
cd ..
```

### Step 3 — Build FlashySpeed

```powershell
go build -o flashyspeed.exe ./cmd/flashyspeed
```

This creates a `flashyspeed.exe` file in the FlashSpeed folder.

### Step 4 — Set a secret key

FlashySpeed needs a secret password to keep your login sessions secure. Run this in PowerShell:

```powershell
$env:FS_JWT_SECRET = "replace-this-with-any-long-random-string-32chars"
```

> 💡 You can type anything here as long as it's at least 32 characters. Something like `my-flashyspeed-secret-key-home-2024` works fine.

### Step 5 — Start FlashySpeed

```powershell
.\flashyspeed.exe
```

You should see:
```
FlashySpeed listening on https://localhost:8080
```

### Step 6 — Open it in your browser

Go to: **https://localhost:8080**

You'll see a warning that says **"Your connection is not private"** — this is normal for a local server. Click **Advanced** then **Proceed to localhost** to continue.

Log in with:
- **Username:** `admin`
- **Password:** `admin`

> ⚠️ Please change this password right away! Go to **⚙ Admin → Users**, click **Edit** next to the admin account, and set a new password.

---

### 💡 Make it easier to start on Windows

Instead of typing those commands every time, save this as a file called `start.ps1` in your FlashSpeed folder:

```powershell
$env:FS_JWT_SECRET = "replace-this-with-your-secret-key-32chars"
.\flashyspeed.exe
```

Then just double-click it (or right-click → **Run with PowerShell**) whenever you want to start FlashySpeed.

---

### 🔁 Start automatically when Windows boots (optional)

If you want FlashySpeed to start every time you turn on your computer:

1. Press **Win + R**, type `shell:startup`, press Enter — a folder opens
2. Right-click inside that folder → **New → Shortcut**
3. In the location field, paste:
   ```
   powershell.exe -WindowStyle Hidden -Command "$env:FS_JWT_SECRET='your-secret-here'; C:\path\to\FlashSpeed\flashyspeed.exe"
   ```
   (replace the path with where you actually put the FlashSpeed folder)
4. Click **Next**, name it `FlashySpeed`, click **Finish**

FlashySpeed will now start silently in the background every time Windows starts.

---

## 🐧 Running on Linux (laptop or home server)

### Step 1 — Download the project

Open a terminal and run:

```bash
git clone https://github.com/Shaf2665/FlashSpeed.git
cd FlashSpeed
```

### Step 2 — Build the web interface

```bash
cd web
npm install
npm run build
cd ..
```

### Step 3 — Build FlashySpeed

```bash
go build -o flashyspeed ./cmd/flashyspeed
```

### Step 4 — Start FlashySpeed

```bash
export FS_JWT_SECRET="replace-this-with-any-long-random-string-32chars"
./flashyspeed
```

You should see:
```
FlashySpeed listening on https://localhost:8080
```

Open **https://localhost:8080** in your browser. Accept the security warning (click **Advanced → Proceed**), then log in with `admin` / `admin`.

> ⚠️ Change the admin password immediately under **⚙ Admin → Users**.

---

### 🔁 Keep it running permanently on Linux (systemd service)

If you want FlashySpeed to run in the background and start automatically on boot, set it up as a service. This is the recommended way to run it on a home server or always-on machine.

#### 1. Copy the program to a system folder

```bash
sudo cp flashyspeed /usr/local/bin/
sudo chmod +x /usr/local/bin/flashyspeed
```

#### 2. Create a folder for FlashySpeed's data

```bash
sudo mkdir -p /var/lib/flashyspeed
```

#### 3. Create the service file

```bash
sudo nano /etc/systemd/system/flashyspeed.service
```

Paste the following into the file, then save it (`Ctrl+O`, Enter, `Ctrl+X`):

```ini
[Unit]
Description=FlashySpeed File Server
After=network.target

[Service]
ExecStart=/usr/local/bin/flashyspeed
Environment=FS_JWT_SECRET=CHANGE_THIS_TO_YOUR_OWN_SECRET_32CHARS_MIN
Environment=FS_DATA_DIR=/var/lib/flashyspeed
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

> ⚠️ Replace `CHANGE_THIS_TO_YOUR_OWN_SECRET_32CHARS_MIN` with your own secret string. Don't leave it as-is.

#### 4. Enable and start the service

```bash
sudo systemctl daemon-reload
sudo systemctl enable flashyspeed
sudo systemctl start flashyspeed
```

#### 5. Check it's working

```bash
sudo systemctl status flashyspeed
```

You should see `active (running)` in green. FlashySpeed is now running and will start automatically every time the machine boots.

Open **https://localhost:8080** (or replace `localhost` with your server's IP address if accessing from another device).

#### Useful commands

```bash
# View live logs
sudo journalctl -u flashyspeed -f

# Stop FlashySpeed
sudo systemctl stop flashyspeed

# Restart after a config change
sudo systemctl restart flashyspeed
```

---

## 🐳 Running with Docker (easiest for servers)

Docker lets you run FlashySpeed in an isolated container with a single command — no need to install Go or Node.js on your server. This is the recommended method for always-on home servers and NAS devices.

### Prerequisites

Install **Docker Desktop** (includes both Docker and Docker Compose):
- **Windows / Mac:** https://www.docker.com/products/docker-desktop/
- **Linux:** https://docs.docker.com/engine/install/ (then also run `sudo apt install docker-compose-plugin`)

To check it's installed, open a terminal and run:
```
docker --version
```

---

### Quick start (4 steps)

#### Step 1 — Download FlashySpeed

```bash
git clone https://github.com/Shaf2665/FlashSpeed.git
cd FlashSpeed
```

#### Step 2 — Create your secret key

FlashySpeed needs a secret string to secure your login sessions. Create a file called `.env` inside the FlashSpeed folder with this one line:

```
FS_JWT_SECRET=replace-this-with-any-random-string-at-least-32-characters-long
```

> 💡 **How to make a good secret:**
> - **Linux/Mac:** Run `openssl rand -hex 32` in a terminal and paste the result
> - **Windows PowerShell:** Run `-join ((1..40) | ForEach-Object { [char](Get-Random -Min 65 -Max 90) })` and paste the result
> - **Or just type something random**, as long as it's 32+ characters (e.g. `my-home-flashyspeed-secret-2024-random-abc`)

> ⚠️ Keep your `.env` file private. It is already in `.gitignore` so it won't be accidentally uploaded to Git.

#### Step 3 — Create a folder for your files

```bash
mkdir files
```

This `files` folder on your computer is where FlashySpeed will read and serve your files from. You can put anything you like in here.

#### Step 4 — Start FlashySpeed

```bash
docker compose up -d
```

Docker will build FlashySpeed (this takes 2–5 minutes the first time — it's downloading and compiling everything). Once done, open your browser and go to:

**https://localhost:8080**

You'll see a warning that says **"Your connection is not private"** — this is normal. FlashySpeed uses a self-signed certificate because it runs locally without a public domain name. Click **Advanced** then **Proceed to localhost** to continue.

Log in with:
- **Username:** `admin`
- **Password:** `admin`

> ⚠️ Change this password right away! Go to **⚙ Admin → Users**, click **Edit** next to the admin account, and set a new password.

---

### Viewing logs

```bash
docker compose logs -f
```

Press `Ctrl+C` to stop. You should see a line like:
```
FlashySpeed listening on https://localhost:8080
```

---

### Adding more file folders

Open `config.docker.yaml` and add more paths under `manual_paths`:

```yaml
storage:
  auto_detect_drives: false
  manual_paths:
    - /files
    - /photos
    - /videos
```

Then open `docker-compose.yml` and add matching lines under `volumes:` so Docker knows which folders on your computer to map:

```yaml
    volumes:
      - flashyspeed_data:/data
      - ./files:/files
      - /path/to/your/photos:/photos
      - /path/to/your/videos:/videos
```

After editing either file, restart FlashySpeed:

```bash
docker compose restart
```

---

### Updating to a new version

```bash
git pull
docker compose up -d --build
```

Your data (database, TLS certificates, uploaded files) is stored separately and is never touched by an update.

---

### Stopping FlashySpeed

```bash
docker compose down
```

This stops the container but keeps all your data safe. Run `docker compose up -d` to start it again.

---

### Where your data lives

| What | Where |
|------|-------|
| Database, TLS certificates | Docker named volume `flashyspeed_data` (managed by Docker) |
| Your files | The `./files` folder next to `docker-compose.yml` |

To back up your files, just copy the `./files` folder. To back up the database, use `docker run --rm -v flashyspeed_data:/data alpine tar czf - /data > backup.tar.gz`.

---

## Accessing from other devices on your network

Once FlashySpeed is running, other devices on the same Wi-Fi or network can access it too.

1. Find your computer's local IP address:
   - **Windows:** Open PowerShell and run `ipconfig` — look for **IPv4 Address** (something like `192.168.1.50`)
   - **Linux:** Run `ip addr` — look for an address like `192.168.1.50`

2. On the other device (phone, tablet, another laptop), open a browser and go to:
   ```
   https://192.168.1.50:8080
   ```
   (replace `192.168.1.50` with your actual IP)

3. Accept the security warning and log in.

---

## Accessing from anywhere in the world (Tailscale)

Want to access your FlashySpeed from outside your home network — like from your phone when you're out? Use **Tailscale**. It's free and creates a secure private tunnel to your server.

1. Log in to FlashySpeed as admin and click **⚙ Admin**
2. Under **Tailscale**, click **⬇ Install Tailscale**
3. Go to https://tailscale.com, create a free account, and get an auth key from **Settings → Keys**
4. Paste the key into FlashySpeed and click **Connect**
5. Your server now has a Tailscale IP (like `100.x.x.x`) — use that address from any of your Tailscale-connected devices

> ℹ️ Tailscale is free for personal use (up to 3 users / 100 devices).

---

## Choosing a folder to store your files

By default, FlashySpeed will auto-detect available drives. You can also tell it exactly which folders to use by creating a config file.

Create a file called `config.yaml` anywhere you like, with this content:

```yaml
storage:
  auto_detect_drives: false
  manual_paths:
    - C:\Users\YourName\Documents    # Windows example
    - /home/yourname/files           # Linux example
```

Then start FlashySpeed with:
```bash
# Windows
.\flashyspeed.exe config.yaml

# Linux
./flashyspeed config.yaml
```

---

## Using FlashySpeed

### Uploading files
Click **⬆ Upload** in the top toolbar and pick a file. Uploads are resumable — if your connection drops halfway through, just upload again and it will pick up where it left off.

### Creating folders
Click **📁 New Folder**, type a name, and press Enter.

### Navigating folders
Click any folder name to open it. A breadcrumb trail at the top (like `🏠 Home › 📁 photos › 📁 2024`) shows where you are — click any part of it to jump back.

### Renaming files
Click the **✏** button on any file row, type the new name, and press Enter.

### Deleting files
Click **🗑** to move a file to trash. Nothing is permanently deleted until you empty the trash. To restore a file, click **🗑 Trash** in the nav bar.

### Searching
Type in the search bar at the top and press Enter. Click **✕ Clear** to go back to browsing.

### Sharing a file
1. Click **🔗 Share** on any file
2. Click **Create Share Link**
3. Copy the link and send it to anyone — they don't need an account to download it

### Selecting multiple files
Tick the checkbox next to files to select them. A bar appears at the top with options to **Delete Selected** or **Download as ZIP**.

### Previewing media
Click **▶ Preview** on any image, video, or audio file to view or play it directly in your browser.

---

## Admin panel

Click **⚙ Admin** in the nav bar (only visible to admin accounts).

### Managing users
- **Create a new user** — fill in the form at the bottom of the Users section
- **Edit a user** — click the Edit button next to their name to change their role or password
- **Set a storage limit** — enter a number of bytes in the quota field (e.g. `10737418240` = 10 GB). Leave at `0` for no limit.
- **Delete a user** — click Delete (their files on disk are kept)

### Storage dashboard
Shows how much space each drive and each user is using, with colour-coded bars (blue = normal, amber = getting full, red = nearly full).

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Browser says "Your connection is not private" | This is normal — click **Advanced → Proceed**. FlashySpeed uses a self-signed certificate by default. |
| Can't connect at all ("connection refused") | Make sure FlashySpeed is still running in your terminal. Try restarting it. |
| "FS_JWT_SECRET env var must be at least 32 bytes" | Your secret key is too short. Use any string of 32 or more characters. |
| Upload fails with "quota exceeded" | Go to **⚙ Admin → Users**, edit the user, and increase or remove their quota. |
| Forgot the admin password | Stop FlashySpeed, delete the `flashyspeed.db` file in the data folder, and restart — this resets everything. |
| Files not showing up | Click **⚙ Admin** and check that your drive/folder is listed. Try restarting FlashySpeed to re-scan. |
| Can't access from another device | Check that both devices are on the same Wi-Fi network and that you're using the right IP address. |

---

## License

MIT — free to use, modify, and share. See [LICENSE](LICENSE) for details.
