(() => {
  const ACCOUNT_RELOAD_DEBOUNCE_MS = 260;
  const ACCOUNT_SCROLL_KEY = "todo-account-scroll-y";
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

  function setAccountSelectOpen(container, shouldOpen) {
    if (!container) {
      return;
    }
    const menu = container.querySelector("[data-account-select-menu]");
    container.classList.toggle("is-open", shouldOpen);
    if (menu) {
      menu.hidden = !shouldOpen;
    }
  }

  function closeAccountSelects(root, except = null) {
    root.querySelectorAll("[data-account-select-root]").forEach((container) => {
      if (container === except) {
        return;
      }
      setAccountSelectOpen(container, false);
    });
  }

  function syncAccountSelect(container) {
    if (!container) {
      return;
    }
    const select = container.querySelector("select");
    const label = container.querySelector("[data-account-select-label]");
    if (!select || !label) {
      return;
    }

    const selectedOption = select.options[select.selectedIndex];
    label.textContent = selectedOption ? selectedOption.textContent.trim() : "";

    container.querySelectorAll("[data-account-select-option]").forEach((optionButton) => {
      const isSelected = optionButton.getAttribute("data-value") === select.value;
      optionButton.classList.toggle("is-selected", isSelected);
      optionButton.setAttribute("aria-pressed", isSelected ? "true" : "false");
    });
  }

  function enhanceAccountSelects(root) {
    root.querySelectorAll("select[data-account-custom-select]").forEach((select) => {
      if (select.dataset.accountSelectReady === "1") {
        return;
      }
      select.dataset.accountSelectReady = "1";

      const wrapper = document.createElement("div");
      wrapper.className = "account-select";
      wrapper.setAttribute("data-account-select-root", "");
      if (select.hasAttribute("data-account-select-center-menu")) {
        wrapper.classList.add("account-select-center-menu");
      }

      const trigger = document.createElement("button");
      trigger.type = "button";
      trigger.className = "account-select-trigger";
      trigger.setAttribute("data-account-select-trigger", "");
      trigger.innerHTML = '<span class="account-select-label" data-account-select-label></span><span class="account-select-caret" aria-hidden="true"></span>';

      const menu = document.createElement("div");
      menu.className = "account-select-menu";
      menu.hidden = true;
      menu.setAttribute("data-account-select-menu", "");

      Array.from(select.options).forEach((option) => {
        const optionButton = document.createElement("button");
        optionButton.type = "button";
        optionButton.className = "account-select-option";
        optionButton.textContent = option.textContent.trim();
        optionButton.setAttribute("data-account-select-option", "");
        optionButton.setAttribute("data-value", option.value);
        optionButton.disabled = option.disabled;
        optionButton.addEventListener("click", () => {
          if (select.value !== option.value) {
            select.value = option.value;
            select.dispatchEvent(new Event("input", { bubbles: true }));
            select.dispatchEvent(new Event("change", { bubbles: true }));
          }
          syncAccountSelect(wrapper);
          setAccountSelectOpen(wrapper, false);
          trigger.focus();
        });
        menu.append(optionButton);
      });

      const parent = select.parentNode;
      if (!parent) {
        return;
      }
      parent.insertBefore(wrapper, select);
      wrapper.append(select, trigger, menu);
      select.classList.add("account-native-select");

      trigger.addEventListener("click", () => {
        if (trigger.disabled) {
          return;
        }
        const shouldOpen = !wrapper.classList.contains("is-open");
        closeAccountSelects(root, wrapper);
        setAccountSelectOpen(wrapper, shouldOpen);
      });

      select.addEventListener("change", () => {
        syncAccountSelect(wrapper);
      });

      syncAccountSelect(wrapper);
    });
  }

  function sanitizeAccountPageInput(input) {
    const totalPages = Number.parseInt(input.dataset.totalPages || "1", 10) || 1;
    const digits = (input.value || "").replace(/[^\d]/g, "");
    const parsed = Number.parseInt(digits || "1", 10);
    const value = Math.min(totalPages, Math.max(1, parsed));
    input.value = String(value);
    return value;
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

  function setWheelValue(root, selector, value) {
    const picker = root.querySelector(selector);
    if (!picker) {
      return;
    }
    if (typeof window.setWheelPickerValue === "function") {
      window.setWheelPickerValue(picker, value);
      return;
    }
    const hiddenInput = picker.querySelector("[data-picker-value]");
    if (hiddenInput) {
      hiddenInput.value = value || "";
    }
  }

  function toggleSingleEditors(root, cards) {
    const isSingle = cards.length === 1;
    root.querySelectorAll("[data-single-only]").forEach((element) => {
      element.hidden = !isSingle;
    });

    const replaceTitleInput = root.querySelector("input[name='replace_title']");
    const scheduleDateInput = root.querySelector("input[name='schedule_date']");
    const deadlineValueInput = root.querySelector("input[name='deadline_value']");
    const timeEditorBlock = root.querySelector("[data-time-editor-block]");
    const timeEditorLabel = root.querySelector("[data-time-editor-label]");
    const dateEditor = root.querySelector("[data-time-editor='date']");
    const datetimeEditor = root.querySelector("[data-time-editor='datetime']");

    if (!replaceTitleInput || !scheduleDateInput || !deadlineValueInput || !timeEditorBlock || !timeEditorLabel || !dateEditor || !datetimeEditor) {
      return;
    }

    if (!isSingle) {
      replaceTitleInput.placeholder = "只在单选时生效";
      replaceTitleInput.value = "";
      scheduleDateInput.value = "";
      deadlineValueInput.value = "";
      timeEditorBlock.hidden = true;
      dateEditor.hidden = true;
      datetimeEditor.hidden = true;
      setWheelValue(root, "#account-schedule-wheel", "");
      setWheelValue(root, "#account-deadline-wheel", "");
      return;
    }

    const card = cards[0];
    replaceTitleInput.placeholder = card.dataset.taskTitle || "单条改名";

    const mode = card.dataset.scheduleMode || "none";
    timeEditorBlock.hidden = mode === "none";
    dateEditor.hidden = mode !== "date";
    datetimeEditor.hidden = mode !== "datetime";

    if (mode === "date") {
      timeEditorLabel.textContent = "单条改日期";
      scheduleDateInput.value = card.dataset.scheduleValue || "";
      deadlineValueInput.value = "";
      setWheelValue(root, "#account-schedule-wheel", card.dataset.scheduleValue || "");
      setWheelValue(root, "#account-deadline-wheel", "");
      return;
    }

    if (mode === "datetime") {
      const deadlineValue = card.dataset.deadlineDate && card.dataset.deadlineTime
        ? `${card.dataset.deadlineDate}T${card.dataset.deadlineTime}`
        : "";
      timeEditorLabel.textContent = "单条改截止时间";
      scheduleDateInput.value = "";
      deadlineValueInput.value = "";
      setWheelValue(root, "#account-schedule-wheel", "");
      setWheelValue(root, "#account-deadline-wheel", deadlineValue);
      return;
    }

    timeEditorLabel.textContent = "单条改时间";
    scheduleDateInput.value = "";
    deadlineValueInput.value = "";
    setWheelValue(root, "#account-schedule-wheel", "");
    setWheelValue(root, "#account-deadline-wheel", "");
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
    const editButton = root.querySelector("[data-account-open-panel='edit']");
    const shareButton = root.querySelector("[data-account-open-panel='share']");
    const deleteButton = root.querySelector("[data-account-submit-action='delete']");

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

    enhanceAccountSelects(root);
    if (typeof window.initializePostponePickers === "function") {
      window.initializePostponePickers(root);
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
        closeAccountSelects(root);
      });
    });

    root.querySelectorAll("[data-account-open-panel]").forEach((button) => {
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

    document.addEventListener("pointerdown", (event) => {
      const target = event.target;
      if (!(target instanceof Node) || !root.contains(target)) {
        closeAccountSelects(root);
        return;
      }
      const selectContainer = target instanceof Element ? target.closest("[data-account-select-root]") : null;
      if (!selectContainer) {
        closeAccountSelects(root);
      }
    });

    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        closeAccountSelects(root);
      }
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

    root.querySelectorAll("[data-account-limit-select]").forEach((select) => {
      select.addEventListener("change", () => {
        try {
          window.sessionStorage.setItem(ACCOUNT_SCROLL_KEY, String(window.scrollY));
        } catch (_error) {
          // Ignore storage failures.
        }
        if (select.form) {
          select.form.requestSubmit();
        }
      });
    });

    root.querySelectorAll("[data-wheel-clear-target]").forEach((button) => {
      button.addEventListener("click", () => {
        const targetId = button.getAttribute("data-wheel-clear-target") || "";
        if (!targetId) {
          return;
        }
        const target = root.querySelector(`#${targetId}`);
        if (!target) {
          return;
        }
        setWheelValue(root, `#${targetId}`, "");
      });
    });

    root.querySelectorAll("[data-account-page-input]").forEach((input) => {
      const submitPage = () => {
        const hiddenPage = input.form?.querySelector("[data-account-page-value]");
        if (hiddenPage) {
          hiddenPage.value = String(sanitizeAccountPageInput(input));
        }
        try {
          window.sessionStorage.setItem(ACCOUNT_SCROLL_KEY, String(window.scrollY));
        } catch (_error) {
          // Ignore storage failures.
        }
        if (input.form) {
          input.form.requestSubmit();
        }
      };

      input.addEventListener("blur", (event) => {
        const nextTarget = event.relatedTarget;
        if (nextTarget instanceof Element && nextTarget.matches("[data-account-page-target]")) {
          return;
        }
        submitPage();
      });

      input.addEventListener("keydown", (event) => {
        if (event.key === "Enter") {
          event.preventDefault();
          submitPage();
        } else if (event.key === "Escape") {
          input.value = input.form?.querySelector("[data-account-page-value]")?.value || input.value;
          input.blur();
        }
      });
    });

    root.querySelectorAll("[data-account-page-target]").forEach((button) => {
      button.addEventListener("click", () => {
        const hiddenPage = button.form?.querySelector("[data-account-page-value]");
        const targetPage = button.getAttribute("data-account-page-target") || "";
        if (hiddenPage && targetPage !== "") {
          hiddenPage.value = targetPage;
        }
        const pageInput = button.form?.querySelector("[data-account-page-input]");
        if (pageInput && targetPage !== "") {
          pageInput.value = targetPage;
        }
      });
    });

    root.querySelectorAll("form[data-account-preserve-scroll]").forEach((preserveForm) => {
      preserveForm.addEventListener("submit", () => {
        try {
          window.sessionStorage.setItem(ACCOUNT_SCROLL_KEY, String(window.scrollY));
        } catch (_error) {
          // Ignore storage failures.
        }
      });
    });

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
    try {
      const rawScroll = window.sessionStorage.getItem(ACCOUNT_SCROLL_KEY);
      if (rawScroll !== null) {
        window.sessionStorage.removeItem(ACCOUNT_SCROLL_KEY);
        const scrollY = Number.parseFloat(rawScroll);
        if (Number.isFinite(scrollY) && scrollY >= 0) {
          window.requestAnimationFrame(() => {
            window.scrollTo({ top: scrollY, behavior: "auto" });
          });
        }
      }
    } catch (_error) {
      // Ignore storage failures.
    }
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
