const byId = (id) => document.getElementById(id);

const homeView = byId("home-view");
const accountView = byId("account-view");
const accountsElement = byId("accounts");
const transactionsElement = byId("transactions");
const commitsElement = byId("commits");
const noticeElement = byId("notice");

let currentAccount = "";
let currentClosed = false;
let commits = [];
let noticeTimer;
let commandPending = false;
let accountRequest = 0;

async function request(path, options = {}) {
  const response = await fetch(path, options);
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || "Request failed");
  return data;
}

function node(tag, className, text) {
  const element = document.createElement(tag);
  if (className) element.className = className;
  if (text !== undefined) element.textContent = text;
  return element;
}

function showNotice(message, kind = "ok") {
  clearNotice();
  noticeElement.textContent = message;
  noticeElement.className = `notice ${kind}`;
  noticeElement.hidden = false;
  if (kind !== "error") {
    noticeTimer = setTimeout(() => { noticeElement.hidden = true; }, 4000);
  }
}

function clearNotice() {
  clearTimeout(noticeTimer);
  noticeElement.hidden = true;
}

async function loadAccounts() {
  const accounts = await request("/api/accounts");
  byId("account-count").textContent = accounts.length;
  accountsElement.replaceChildren();
  if (accounts.length === 0) {
    accountsElement.append(node("p", "empty", "No event streams yet."));
    return;
  }
  for (const account of accounts) {
    const row = node("div", `account-row${account.closed ? " closed" : ""}`);
    const details = node("div", "account-details");
    details.append(node("strong", "", account.id));
    details.append(node("span", "", `${account.balance} / ${account.closed ? "closed" : "open"}`));
    const button = node("button", "button quiet", "View");
    button.type = "button";
    button.setAttribute("aria-label", `View account ${account.id}`);
    button.addEventListener("click", () => {
      openAccount(account.id).catch((error) => showNotice(error.message, "error"));
    });
    row.append(details, button);
    accountsElement.append(row);
  }
}

async function openAccount(accountID) {
  const requestID = ++accountRequest;
  let account;
  try {
    account = await request(`/api/account?id=${encodeURIComponent(accountID)}`);
  } catch (error) {
    if (requestID === accountRequest) throw error;
    return;
  }
  if (requestID !== accountRequest) return;
  currentAccount = account.id;
  homeView.hidden = true;
  accountView.hidden = false;
  renderAccount(account);
  clearNotice();
  byId("current-account").focus();
}

function renderAccount(account) {
  currentClosed = account.closed;
  byId("current-account").textContent = account.id;
  byId("current-balance").textContent = account.balance;
  byId("account-status").textContent = account.closed ? "Closed" : "Open";
  byId("account-status").className = `status${account.closed ? " closed" : ""}`;
  setCommandControlsDisabled(account.closed);

  transactionsElement.replaceChildren();
  if (account.transactions.length === 0) {
    const row = document.createElement("tr");
    const cell = node("td", "empty-cell", "No transactions yet.");
    cell.colSpan = 3;
    row.append(cell);
    transactionsElement.append(row);
    return;
  }
  for (const transaction of account.transactions) {
    const row = document.createElement("tr");
    row.append(node("td", "", new Date(transaction.at).toLocaleString()));
    row.append(node("td", transaction.amount.startsWith("-") ? "negative" : "positive", transaction.amount));
    row.append(node("td", "numeric", transaction.balance));
    transactionsElement.append(row);
  }
}

async function runCommand(type) {
  if (commandPending) return;
  commandPending = true;
  setCommandControlsDisabled(true);
  const payload = { accountId: currentAccount, type };
  if (type !== "close") payload.amount = byId("amount").value;
  try {
    const result = await request("/api/commands", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    renderAccount(result.account);
    commits.unshift(result.commit);
    renderCommits();
    showNotice(`${type[0].toUpperCase()}${type.slice(1)} committed.`);
  } catch (error) {
    showNotice(error.message, "error");
  } finally {
    commandPending = false;
    setCommandControlsDisabled(currentClosed);
  }
}

function setCommandControlsDisabled(disabled) {
  const commandsDisabled = disabled || commandPending;
  byId("deposit").disabled = commandsDisabled;
  byId("withdraw").disabled = commandsDisabled;
  byId("close-account").disabled = commandsDisabled;
  byId("switch-account").disabled = commandPending;
}

async function loadCommits() {
  commits = await request("/api/commits");
  renderCommits();
}

function renderCommits() {
  byId("commit-count").textContent = commits.length;
  commitsElement.replaceChildren();
  if (commits.length === 0) {
    commitsElement.append(node("p", "empty", "Commands will appear here as events."));
    return;
  }
  for (const commit of commits) {
    const article = node("article", "commit");
    const heading = node("div", "commit-heading");
    heading.append(node("strong", "", commit.stream));
    heading.append(node("span", "", `${commit.events.length} event${commit.events.length === 1 ? "" : "s"}`));
    article.append(heading);
    for (const event of commit.events) {
      const eventBlock = node("div", "event");
      eventBlock.append(node("span", "event-type", event.type));
      const payload = node("pre", "");
      payload.textContent = JSON.stringify(event, null, 2);
      eventBlock.append(payload);
      article.append(eventBlock);
    }
    commitsElement.append(article);
  }
}

byId("open-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const accountID = byId("account-id").value;
  if (!accountID.trim()) {
    showNotice("Account name cannot be blank.", "error");
    return;
  }
  try {
    await openAccount(accountID);
  } catch (error) {
    showNotice(error.message, "error");
  }
});

byId("switch-account").addEventListener("click", async () => {
  accountRequest++;
  currentAccount = "";
  accountView.hidden = true;
  homeView.hidden = false;
  byId("account-id").disabled = true;
  byId("open-account").disabled = true;
  accountsElement.replaceChildren(node("p", "empty", "Loading accounts..."));
  try {
    await loadAccounts();
    clearNotice();
  } catch (error) {
    accountsElement.replaceChildren(node("p", "empty", "Accounts could not be loaded."));
    showNotice(error.message, "error");
  } finally {
    byId("account-id").disabled = false;
    byId("open-account").disabled = false;
    byId("account-id").focus();
  }
});
byId("transaction-form").addEventListener("submit", (event) => {
  event.preventDefault();
  runCommand("deposit");
});
byId("withdraw").addEventListener("click", () => runCommand("withdraw"));
byId("close-account").addEventListener("click", () => runCommand("close"));

async function initialize() {
  await loadCommits();
  await loadAccounts();
}

initialize()
  .catch((error) => showNotice(error.message, "error"))
  .finally(() => {
    byId("account-id").disabled = false;
    byId("open-account").disabled = false;
  });
