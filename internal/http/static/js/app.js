// The dialogs of the app.
//
// A screen asks htmx for a dialog and htmx swaps it into a slot on the page.
// This file opens what arrived. A dialog the browser opens as modal carries the
// backdrop, the focus trap and the Escape key by itself, so nothing here does.
(() => {
  "use strict";

  // opened reports whether the swapped element delivered a dialog, and opens
  // the one it delivered. A swap that carries no dialog leaves the page alone.
  function opened(slot) {
    const dialog = slot.querySelector("dialog");
    if (!dialog || dialog.open) {
      return false;
    }

    // The slot empties when the dialog closes, so the next swap starts clean.
    dialog.addEventListener("close", () => slot.replaceChildren());

    // The backdrop belongs to the dialog element, so a click that lands on the
    // dialog itself and not on its content landed beside the content.
    dialog.addEventListener("click", (event) => {
      if (event.target === dialog) {
        dialog.close();
      }
    });

    dialog.showModal();
    return true;
  }

  document.addEventListener("htmx:afterSwap", (event) => opened(event.target));

  // The buttons that close the dialog they stand in.
  document.addEventListener("click", (event) => {
    const button = event.target.closest("[data-close-dialog]");
    if (button) {
      button.closest("dialog").close();
    }
  });
})();
