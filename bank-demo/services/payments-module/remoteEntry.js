// Diverge Demo — Payments Module (Simulated Module Federation Remote)
// This file is served by a Go container and loaded dynamically by the shell.
// It registers itself into window.__divergeModules for the shell to discover.

(function () {
  "use strict";

  const MODULE_NAME = "payments";

  // Module metadata — the shell reads this after loading
  const meta = {
    name: MODULE_NAME,
    version: "__APP_VERSION__", // replaced at build time by sed
    render: renderPaymentsModule,
  };

  // ── Render function ──────────────────────────────────────────
  function renderPaymentsModule(container, options) {
    const previewId = (options && options.previewId) || "";
    const gatewayBase = (options && options.gatewayUrl) || "";
    const isPreview = meta.version !== "__APP_VERSION__";

    container.innerHTML = buildShell(isPreview);
    fetchPayments(container, gatewayBase, previewId);
    fetchTransactions(container, gatewayBase, previewId, isPreview);
  }

  function buildShell(isPreview) {
    const badge = isPreview
      ? '<span style="background:#03dac6;color:#000;padding:2px 8px;border-radius:4px;font-size:0.75em;margin-left:8px;font-weight:600;">PREVIEW ' + meta.version + '</span>'
      : '<span style="background:#555;color:#aaa;padding:2px 8px;border-radius:4px;font-size:0.75em;margin-left:8px;">BASELINE</span>';

    return (
      '<div style="background:#1e1e1e;border-radius:8px;padding:20px;margin-bottom:20px;' +
      (isPreview ? "border:1px solid #03dac6;box-shadow:0 0 12px rgba(3,218,198,0.15);" : "box-shadow:0 4px 6px rgba(0,0,0,0.3);") +
      '">' +
      '<h2 style="color:#bb86fc;margin-top:0;">Payments Module ' + badge + "</h2>" +
      '<div id="pm-summary" style="display:flex;gap:20px;margin-bottom:16px;">' +
      '<div style="background:#333;padding:12px 20px;border-radius:8px;flex:1;text-align:center;">' +
      '<div style="color:#aaa;font-size:0.8em;">Total Payments</div>' +
      '<div id="pm-count" style="font-size:1.8em;font-weight:bold;color:#fff;">--</div>' +
      "</div>" +
      '<div style="background:#333;padding:12px 20px;border-radius:8px;flex:1;text-align:center;">' +
      '<div style="color:#aaa;font-size:0.8em;">Total Volume</div>' +
      '<div id="pm-volume" style="font-size:1.8em;font-weight:bold;color:#fff;">$--</div>' +
      "</div>" +
      (isPreview
        ? '<div style="background:#1a3330;padding:12px 20px;border-radius:8px;flex:1;text-align:center;border:1px solid #03dac6;">' +
          '<div style="color:#03dac6;font-size:0.8em;">Total Fees</div>' +
          '<div id="pm-fees" style="font-size:1.8em;font-weight:bold;color:#03dac6;">$--</div>' +
          "</div>"
        : "") +
      "</div>" +
      '<table style="width:100%;border-collapse:collapse;">' +
      "<thead><tr>" +
      '<th style="text-align:left;padding:10px;border-bottom:1px solid #333;color:#aaa;">ID</th>' +
      '<th style="text-align:left;padding:10px;border-bottom:1px solid #333;color:#aaa;">From</th>' +
      '<th style="text-align:left;padding:10px;border-bottom:1px solid #333;color:#aaa;">To</th>' +
      '<th style="text-align:right;padding:10px;border-bottom:1px solid #333;color:#aaa;">Amount</th>' +
      (isPreview ? '<th style="text-align:right;padding:10px;border-bottom:1px solid #333;color:#03dac6;">Fee ✨</th>' : "") +
      '<th style="text-align:left;padding:10px;border-bottom:1px solid #333;color:#aaa;">Status</th>' +
      "</tr></thead>" +
      '<tbody id="pm-transactions"><tr><td colspan="' + (isPreview ? 6 : 5) + '" style="padding:10px;color:#666;">Loading transactions...</td></tr></tbody>' +
      "</table>" +
      "</div>"
    );
  }

  function fetchPayments(container, base, previewId) {
    var headers = {};
    if (previewId) headers["x-preview-id"] = previewId;
    fetch(base + "/api/payments", { headers: headers })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        var countEl = container.querySelector("#pm-count");
        if (countEl && data.payments) countEl.textContent = data.payments.length;
      })
      .catch(function () {});
  }

  function fetchTransactions(container, base, previewId, isPreview) {
    var headers = {};
    if (previewId) headers["x-preview-id"] = previewId;
    fetch(base + "/api/payments/transactions", { headers: headers })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (!data.transactions) return;

        var totalVolume = 0;
        var totalFees = 0;
        var rows = "";

        data.transactions.forEach(function (tx) {
          var amount = parseFloat(tx.amount) || 0;
          var fee = parseFloat(tx.fee) || 0;
          totalVolume += amount;
          totalFees += fee;

          rows +=
            "<tr>" +
            '<td style="padding:10px;border-bottom:1px solid #333;">' + tx.id + "</td>" +
            '<td style="padding:10px;border-bottom:1px solid #333;">' + (tx.from_account || tx.from || "-") + "</td>" +
            '<td style="padding:10px;border-bottom:1px solid #333;">' + (tx.to_account || tx.to || "-") + "</td>" +
            '<td style="padding:10px;border-bottom:1px solid #333;text-align:right;">$' + amount.toFixed(2) + "</td>" +
            (isPreview
              ? '<td style="padding:10px;border-bottom:1px solid #333;text-align:right;color:#03dac6;">$' + fee.toFixed(2) + "</td>"
              : "") +
            '<td style="padding:10px;border-bottom:1px solid #333;">' +
            '<span style="background:' + (tx.status === "completed" ? "#2e7d32" : "#f57f17") + ";padding:2px 8px;border-radius:4px;font-size:0.8em;\">" +
            (tx.status || "pending") +
            "</span></td>" +
            "</tr>";
        });

        var tbody = container.querySelector("#pm-transactions");
        if (tbody) tbody.innerHTML = rows || '<tr><td colspan="5" style="padding:10px;color:#666;">No transactions</td></tr>';

        var volumeEl = container.querySelector("#pm-volume");
        if (volumeEl) volumeEl.textContent = "$" + totalVolume.toFixed(2);

        var countEl = container.querySelector("#pm-count");
        if (countEl) countEl.textContent = data.transactions.length;

        if (isPreview) {
          var feesEl = container.querySelector("#pm-fees");
          if (feesEl) feesEl.textContent = "$" + totalFees.toFixed(2);
        }
      })
      .catch(function (err) {
        var tbody = container.querySelector("#pm-transactions");
        if (tbody) tbody.innerHTML = '<tr><td colspan="5" style="padding:10px;color:#f44;">Failed to load: ' + err.message + "</td></tr>";
      });
  }

  // ── Register module ──────────────────────────────────────────
  window.__divergeModules = window.__divergeModules || {};
  window.__divergeModules[MODULE_NAME] = meta;
})();
