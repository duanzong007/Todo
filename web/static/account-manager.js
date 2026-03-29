(() => {
  const ACCOUNT_RELOAD_DEBOUNCE_MS = 260;
  let accountReloadTimer = 0;
  let accountSubmitInFlight = false;

  function accountRoot() {
    return document.querySelector("[data-account-page]");
  }

  function selectedCards(root) {
    return Array.from(root.querySelectorAll("[data-account-task-checkbox]:checked"))
      .map((checkbox) => checkbox.closest("[data-account-task]"))
      .filter(Boolean);
  }

  function syncModalState(root) {
    const hasOpenModal = Array.from(root.querySelectorAll(".account-modal[data-account-panel]"))
      .some((panel) => !panel.hidden);
    document.body.classList.toggle("account-modal-open", hasOpenModal);
  }

  function setPanelState(root, name, shouldOpen) {
    const panel = root.querySelector(`[data-account-panel="${name}"]`);
    if (!panel) {
      return;
    }
    panel.hidden = !shouldOpen;
    syncModalState(root);
  }

  function togglePanel(root, name) {
    const panel = root.querySelector(`[data-account-panel="${name}"]`);
    if (!panel) {
      return;
    }
    setPanelState(root, name, panel.hidden);
  }

  function updateEditSummary(root, cards) {
    const summary = root.querySelector("[data-edit-summary]");
    if (!summary) {
      return;
    }

    if (cards.length === 0) {
      summary.textContent = "选中任务后可以修改。";
      return;
    }

    if (cards.length > 1) {
      const allTodo = cards.every((card) => (card.dataset.taskType || "") === "todo");
      const allSchedule = cards.every((card) => (card.dataset.taskType || "") === "schedule");
      const allDDL = cards.every((card) => (card.dataset.taskType || "") === "ddl");

      if (allTodo) {
        summary.textContent = "当前是 Todo 批量编辑，可统一改前后缀和星级。";
        return;
      }
      if (allSchedule) {
        summary.textContent = "当前是日程批量编辑，可统一改前后缀和星级。";
        return;
      }
      if (allDDL) {
        summary.textContent = "当前是 DDL 批量编辑，可统一改前后缀和星级。";
        return;
      }

      summary.textContent = "混合类型任务只支持批量改前后缀和星级。";
      return;
    }

    switch (cards[0].dataset.taskType || "") {
      case "schedule":
        summary.textContent = "当前是单条日程，可修改标题、星级和日期。";
        return;
      case "ddl":
        summary.textContent = "当前是单条 DDL，可修改标题、星级和截止时间。";
        return;
      default:
        summary.textContent = "当前是单条 Todo，可修改标题和星级。";
    }
  }

  function toggleSingleEditors(root, cards) {
    const isSingle = cards.length === 1;
    root.querySelectorAll("[data-single-only]").forEach((element) => {
      element.hidden = !isSingle;
    });

    const replaceTitleInput = root.querySelector("input[name='replace_title']");
    const scheduleDateInput = root.querySelector("input[name='schedule_date']");
    const deadlineDateInput = root.querySelector("input[name='deadline_date']");
    const deadlineTimeInput = root.querySelector("input[name='deadline_time']");
    const dateEditor = root.querySelector("[data-time-editor='date']");
    const datetimeEditor = root.querySelector("[data-time-editor='datetime']");
    const noneEditor = root.querySelector("[data-time-editor='none']");

    if (!replaceTitleInput || !scheduleDateInput || !deadlineDateInput || !deadlineTimeInput || !dateEditor || !datetimeEditor || !noneEditor) {
      return;
    }

    if (!isSingle) {
      replaceTitleInput.placeholder = "只在单选时生效";
      replaceTitleInput.value = "";
      scheduleDateInput.value = "";
      deadlineDateInput.value = "";
      deadlineTimeInput.value = "";
      dateEditor.hidden = true;
      datetimeEditor.hidden = true;
      noneEditor.hidden = true;
      return;
    }

    const card = cards[0];
    replaceTitleInput.placeholder = card.dataset.taskTitle || "单条改名";

    const mode = card.dataset.scheduleMode || "none";
    dateEditor.hidden = mode !== "date";
    datetimeEditor.hidden = mode !== "datetime";
    noneEditor.hidden = mode !== "none";

    if (mode === "date") {
      scheduleDateInput.value = card.dataset.scheduleValue || "";
      deadlineDateInput.value = "";
      deadlineTimeInput.value = "";
      return;
    }

    if (mode === "datetime") {
      scheduleDateInput.value = "";
      deadlineDateInput.value = card.dataset.deadlineDate || "";
      deadlineTimeInput.value = card.dataset.deadlineTime || "";
      return;
    }

    scheduleDateInput.value = "";
    deadlineDateInput.value = "";
    deadlineTimeInput.value = "";
  }

  function updateAccountSelectionState(root, form) {
    const cards = selectedCards(root);
    const ids = cards.map((card) => card.dataset.taskId || "").filter(Boolean);
    const allCards = Array.from(root.querySelectorAll("[data-account-task]"));
    const selectedIdsInput = form.querySelector("[data-selected-task-ids]");
    if (selectedIdsInput) {
      selectedIdsInput.value = ids.join(",");
    }

    const selectionCopy = form.querySelector("[data-selection-copy]");
    if (selectionCopy) {
      if (ids.length === 0) {
        selectionCopy.textContent = "尚未选择任务";
      } else if (ids.length === 1) {
        selectionCopy.textContent = "已选择 1 条任务";
      } else {
        selectionCopy.textContent = `已选择 ${ids.length} 条任务`;
      }
    }

    const selectAll = form.querySelector("[data-select-all-tasks]");
    if (selectAll) {
      selectAll.checked = ids.length > 0 && ids.length === allCards.length;
      selectAll.indeterminate = ids.length > 0 && ids.length < allCards.length;
    }

    toggleSingleEditors(root, cards);
    updateEditSummary(root, cards);

    const allOwned = ids.length > 0 && cards.every((card) => card.dataset.isOwner === "1");
    const editButton = form.querySelector("[data-account-open-panel='edit']");
    const shareButton = form.querySelector("[data-account-open-panel='share']");
    const deleteButton = form.querySelector("[data-account-submit-action='delete']");

    if (editButton) {
      editButton.disabled = ids.length === 0;
    }
    if (shareButton) {
      shareButton.disabled = !allOwned;
    }
    if (deleteButton) {
      deleteButton.disabled = !allOwned;
    }

    if (ids.length === 0) {
      setPanelState(root, "edit", false);
      setPanelState(root, "share", false);
    } else if (!allOwned) {
      setPanelState(root, "share", false);
    }
  }

  function bindAccountManager(root) {
    if (!root || root.dataset.accountManagerBound === "1") {
      return;
    }
    root.dataset.accountManagerBound = "1";

    const form = root.querySelector("[data-account-actions-form]");
    if (!form) {
      return;
    }

    root.querySelectorAll("[data-account-toggle]").forEach((button) => {
      button.addEventListener("click", () => {
        togglePanel(root, button.getAttribute("data-account-toggle") || "");
      });
    });

    root.querySelectorAll("[data-account-close-panel]").forEach((button) => {
      button.addEventListener("click", () => {
        const target = button.getAttribute("data-account-close-panel") || "";
        setPanelState(root, target, false);
      });
    });

    form.querySelectorAll("[data-account-open-panel]").forEach((button) => {
      button.addEventListener("click", () => {
        if (button.disabled) {
          return;
        }

        const target = button.getAttribute("data-account-open-panel") || "";
        const panel = root.querySelector(`[data-account-panel="${target}"]`);
        if (!panel) {
          return;
        }

        const shouldOpen = panel.hidden;
        setPanelState(root, target, shouldOpen);
        const other = target === "edit" ? "share" : "edit";
        setPanelState(root, other, false);
      });
    });

    root.querySelectorAll("[data-account-task-checkbox]").forEach((checkbox) => {
      checkbox.addEventListener("change", () => {
        updateAccountSelectionState(root, form);
      });
    });

    const selectAll = form.querySelector("[data-select-all-tasks]");
    if (selectAll) {
      selectAll.addEventListener("change", () => {
        root.querySelectorAll("[data-account-task-checkbox]").forEach((checkbox) => {
          checkbox.checked = selectAll.checked;
        });
        updateAccountSelectionState(root, form);
      });
    }

    root.querySelectorAll("[data-account-submit-action]").forEach((button) => {
      button.addEventListener("click", (event) => {
        const action = button.getAttribute("data-account-submit-action") || "apply";
        const actionInput = form.querySelector("[data-account-action]");
        if (actionInput) {
          actionInput.value = action;
        }

        if (action === "delete") {
          const ids = (form.querySelector("[data-selected-task-ids]")?.value || "").trim();
          if (!ids) {
            event.preventDefault();
            return;
          }
          if (!window.confirm("确定删除选中的任务吗？这个操作不能撤销。")) {
            event.preventDefault();
          }
        }
      });
    });

    form.addEventListener("submit", (event) => {
      const ids = (form.querySelector("[data-selected-task-ids]")?.value || "").trim();
      const action = (form.querySelector("[data-account-action]")?.value || "apply").trim();
      if (!ids) {
        event.preventDefault();
        return;
      }
      if (action === "share") {
        const checkedUsers = form.querySelectorAll("input[name='share_user_id']:checked");
        if (checkedUsers.length === 0) {
          event.preventDefault();
          return;
        }
      }
      accountSubmitInFlight = true;
    });

    const clearImportance = root.querySelector("[data-clear-importance]");
    if (clearImportance) {
      clearImportance.addEventListener("click", () => {
        form.querySelectorAll("input[name='importance']").forEach((input) => {
          input.checked = false;
        });
      });
    }

    const shareSearch = form.querySelector("[data-share-user-search]");
    if (shareSearch) {
      shareSearch.addEventListener("input", () => {
        const query = shareSearch.value.trim().toLowerCase();
        form.querySelectorAll("[data-share-user-option]").forEach((option) => {
          const haystack = (option.getAttribute("data-search-text") || "").toLowerCase();
          option.hidden = query !== "" && !haystack.includes(query);
        });
      });
    }

    setPanelState(root, "filter", false);
    setPanelState(root, "edit", false);
    setPanelState(root, "share", false);
    updateAccountSelectionState(root, form);
    syncModalState(root);
  }

  function scheduleAccountReload() {
    if (accountSubmitInFlight) {
      return;
    }
    if (accountReloadTimer) {
      window.clearTimeout(accountReloadTimer);
    }
    accountReloadTimer = window.setTimeout(() => {
      window.location.reload();
    }, ACCOUNT_RELOAD_DEBOUNCE_MS);
  }

  function bindAccountRealtime() {
    const root = accountRoot();
    if (!root || root.dataset.accountRealtimeBound === "1") {
      return;
    }
    root.dataset.accountRealtimeBound = "1";

    if (typeof EventSource === "function") {
      try {
        const stream = new EventSource("/events");
        stream.addEventListener("dashboard", () => {
          if (document.hidden) {
            return;
          }
          scheduleAccountReload();
        });
        stream.onerror = () => {};
        window.addEventListener("beforeunload", () => {
          stream.close();
        }, { once: true });
      } catch (_error) {
        // Ignore realtime failures.
      }
    }

    document.addEventListener("visibilitychange", () => {
      if (!document.hidden) {
        scheduleAccountReload();
      }
    });
  }

  function initializeAccountManager(scope = document) {
    const page = scope.querySelector("[data-account-page]");
    if (page) {
      bindAccountManager(page);
    }
    bindAccountRealtime();
  }

  window.initializeAccountManager = initializeAccountManager;
  document.addEventListener("DOMContentLoaded", () => {
    initializeAccountManager(document);
    document.addEventListener("keydown", (event) => {
      if (event.key !== "Escape") {
        return;
      }
      const root = accountRoot();
      if (!root) {
        return;
      }
      setPanelState(root, "filter", false);
      setPanelState(root, "edit", false);
    });
  });
})();
