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
    var colCount = isPreview ? 6 : 5;
    fetch(base + "/api/payments/transactions", { headers: headers })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (!data.transactions) return;

        var totalVolume = 0;
        var totalFees = 0;
        var tbody = container.querySelector("#pm-transactions");
        if (!tbody) return;
        tbody.innerHTML = "";

        data.transactions.forEach(function (tx) {
          var amount = parseFloat(tx.amount) || 0;
          var fee = parseFloat(tx.fee) || 0;
          totalVolume += amount;
          totalFees += fee;

          var tr = document.createElement("tr");
          var cellStyle = "padding:10px;border-bottom:1px solid #333;";

          var cells = [
            { text: tx.id },
            { text: tx.from_account || tx.from || "-" },
            { text: tx.to_account || tx.to || "-" },
            { text: "$" + amount.toFixed(2), style: cellStyle + "text-align:right;" }
          ];

          cells.forEach(function (c) {
            var td = document.createElement("td");
            td.style.cssText = c.style || cellStyle;
            td.textContent = c.text;
            tr.appendChild(td);
          });

          if (isPreview) {
            var feeTd = document.createElement("td");
            feeTd.style.cssText = cellStyle + "text-align:right;color:#03dac6;";
            feeTd.textContent = "$" + fee.toFixed(2);
            tr.appendChild(feeTd);
          }

          var statusTd = document.createElement("td");
          statusTd.style.cssText = cellStyle;
          var span = document.createElement("span");
          span.style.cssText = "background:" + (tx.status === "completed" ? "#2e7d32" : "#f57f17") + ";padding:2px 8px;border-radius:4px;font-size:0.8em;";
          span.textContent = tx.status || "pending";
          statusTd.appendChild(span);
          tr.appendChild(statusTd);

          tbody.appendChild(tr);
        });

        if (data.transactions.length === 0) {
          var emptyTr = document.createElement("tr");
          var emptyTd = document.createElement("td");
          emptyTd.colSpan = colCount;
          emptyTd.style.cssText = "padding:10px;color:#666;";
          emptyTd.textContent = "No transactions";
          emptyTr.appendChild(emptyTd);
          tbody.appendChild(emptyTr);
        }

        var volumeEl = container.querySelector("#pm-volume");
        if (volumeEl) volumeEl.textContent = "$" + totalVolume.toFixed(2);

        if (isPreview) {
          var feesEl = container.querySelector("#pm-fees");
          if (feesEl) feesEl.textContent = "$" + totalFees.toFixed(2);
        }
      })
      .catch(function (err) {
        var tbody = container.querySelector("#pm-transactions");
        if (tbody) {
          tbody.innerHTML = "";
          var tr = document.createElement("tr");
          var td = document.createElement("td");
          td.colSpan = colCount;
          td.style.cssText = "padding:10px;color:#f44;";
          td.textContent = "Failed to load transactions";
          tr.appendChild(td);
          tbody.appendChild(tr);
        }
      });
  }

  // ── Register module ──────────────────────────────────────────
  window.__divergeModules = window.__divergeModules || {};
  window.__divergeModules[MODULE_NAME] = meta;
})();
