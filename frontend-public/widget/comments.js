/* VexGo comment widget — self-contained and framework-free.
 *
 * Public pages are server-side rendered, so interactive parts (the comment
 * section) are provided by this widget. Any theme can adopt it with two lines:
 *
 *   <div id="vexgo-comments" data-post-id="{{.Post.ID}}"></div>
 *   <script src="/theme-assets/comments.js" defer></script>
 *
 * Note: /theme-assets/... already resolves into the theme's assets/ dir, so
 * the file is referenced without an extra assets/ segment.
 *
 * The widget talks to the public comment API (/api/comments/...), reuses the
 * admin SPA's localStorage session ("token" and "user" keys), and styles
 * itself with inline styles so it renders correctly in any theme regardless
 * of the theme's CSS framework. All user content is rendered via textContent,
 * never innerHTML.
 *
 * i18n: the post template passes the resolved language via
 * <div id="vexgo-comments" data-post-id="..." data-lang="zh">. The widget
 * carries a built-in copy of the comments.* strings for the languages the
 * default theme ships; third-party themes replacing this file own their copy.
 * Unknown languages fall back to English per key.
 */
(function () {
  "use strict";

  var container = document.getElementById("vexgo-comments");
  if (!container) return;
  var postId = container.getAttribute("data-post-id");
  if (!postId) return;

  var STRINGS = {
    en: {
      title: "Comments ",
      empty: "No comments yet.",
      loginPrompt: "Please log in to comment.",
      loginBtn: "Log in",
      placeholder: "Write a comment...",
      submit: "Submit",
      moderationNote: "Thanks! Your comment is awaiting moderation.",
      deleteBtn: "Delete",
      deleteConfirm: "Delete this comment?",
      loadError: "Failed to load comments.",
      postError: "Failed to post comment",
      deleteError: "Failed to delete comment",
      likeError: "Failed to update like",
    },
    zh: {
      title: "评论 ",
      empty: "暂无评论。",
      loginPrompt: "请先登录再评论。",
      loginBtn: "登录",
      placeholder: "写下你的评论...",
      submit: "提交",
      moderationNote: "感谢！你的评论正在等待审核。",
      deleteBtn: "删除",
      deleteConfirm: "删除这条评论？",
      loadError: "评论加载失败。",
      postError: "评论发布失败",
      deleteError: "评论删除失败",
      likeError: "点赞更新失败",
    },
  };

  var lang = (container.getAttribute("data-lang") || "en").toLowerCase();
  if (lang.indexOf("-") >= 0) lang = lang.slice(0, lang.indexOf("-"));
  if (lang.indexOf("_") >= 0) lang = lang.slice(0, lang.indexOf("_"));
  var dict = STRINGS[lang] || STRINGS.en;
  function t(key) {
    if (dict[key] != null && dict[key] !== "") return dict[key];
    if (STRINGS.en[key] != null) return STRINGS.en[key];
    return key;
  }

  var API = "/api";

  var styles = {
    title: "font-size:1.5rem;font-weight:700;margin:0 0 1rem;",
    count: "font-size:1rem;font-weight:600;opacity:.7;",
    form: "border:1px solid rgba(128,128,128,.35);border-radius:.5rem;padding:1rem;margin-bottom:1.5rem;",
    textarea:
      "width:100%;min-height:6rem;box-sizing:border-box;padding:.5rem .75rem;border:1px solid rgba(128,128,128,.35);border-radius:.375rem;font:inherit;",
    row: "display:flex;align-items:center;justify-content:space-between;gap:.5rem;margin-top:.5rem;",
    button:
      "padding:.5rem 1rem;border-radius:.375rem;border:1px solid rgba(128,128,128,.35);background:rgba(128,128,128,.12);cursor:pointer;font:inherit;",
    login:
      "border:1px solid rgba(128,128,128,.35);border-radius:.5rem;padding:1rem;text-align:center;margin-bottom:1.5rem;",
    list: "list-style:none;margin:0;padding:0;",
    item: "border:1px solid rgba(128,128,128,.35);border-radius:.5rem;padding:1rem;margin-bottom:.75rem;",
    meta: "display:flex;align-items:center;justify-content:space-between;gap:.5rem;margin-bottom:.25rem;",
    author: "display:flex;align-items:center;gap:.5rem;min-width:0;",
    avatar:
      "width:2.25rem;height:2.25rem;border-radius:9999px;object-fit:cover;flex-shrink:0;",
    name: "font-weight:600;",
    time: "font-size:.8rem;opacity:.7;margin-left:.25rem;",
    body: "margin:0;white-space:pre-wrap;word-break:break-word;",
    empty: "text-align:center;padding:2rem 0;opacity:.7;",
    error: "color:#b91c1c;font-size:.85rem;margin:.5rem 0 0;",
    note: "margin:.5rem 0 0;font-size:.85rem;opacity:.85;",
    counter: "font-size:.8rem;opacity:.7;",
    deleteBtn:
      "border:0;background:none;cursor:pointer;font-size:.8rem;opacity:.6;text-decoration:underline;",
  };

  function el(tag, attrs, children) {
    var node = document.createElement(tag);
    for (var key in attrs) {
      if (key === "style") {
        node.setAttribute("style", attrs[key]);
      } else if (key === "text") {
        node.textContent = attrs[key];
      } else {
        node.setAttribute(key, attrs[key]);
      }
    }
    (children || []).forEach(function (child) {
      if (typeof child === "string") {
        node.appendChild(document.createTextNode(child));
      } else if (child) {
        node.appendChild(child);
      }
    });
    return node;
  }

  function fmtDate(iso) {
    try {
      return new Date(iso).toLocaleString();
    } catch (e) {
      return "";
    }
  }

  function currentUser() {
    try {
      var raw = localStorage.getItem("user");
      return raw ? JSON.parse(raw) : null;
    } catch (e) {
      return null;
    }
  }

  function canDelete(comment) {
    var u = currentUser();
    if (!u) return false;
    if (String(comment.userId) === String(u.id)) return true;
    return u.role === "admin" || u.role === "super_admin";
  }

  function loadComments(callback) {
    fetch(API + "/comments/post/" + encodeURIComponent(postId))
      .then(function (res) {
        if (!res.ok) throw new Error("HTTP " + res.status);
        return res.json();
      })
      .then(function (data) {
        callback(data.comments || []);
      })
      .catch(function () {
        renderError();
      });
  }

  function renderList(comments) {
    list.textContent = "";
    count.textContent = "(" + comments.length + ")";
    if (comments.length === 0) {
      list.appendChild(el("li", { style: styles.empty, text: t("empty") }));
      return;
    }
    comments.forEach(function (comment) {
      var meta = el("div", { style: styles.meta });
      var author = el("div", { style: styles.author });
      if (comment.author && comment.author.avatar) {
        author.appendChild(
          el("img", {
            style: styles.avatar,
            src: comment.author.avatar,
            alt: "",
          }),
        );
      }
      author.appendChild(
        el("span", {
          style: styles.name,
          text: (comment.author && comment.author.username) || "anonymous",
        }),
      );
      if (comment.createdAt) {
        author.appendChild(
          el("span", { style: styles.time, text: fmtDate(comment.createdAt) }),
        );
      }
      meta.appendChild(author);
      if (canDelete(comment)) {
        meta.appendChild(
          el("button", {
            style: styles.deleteBtn,
            type: "button",
            text: t("deleteBtn"),
            onclick: function () {
              deleteComment(comment);
            },
          }),
        );
      }
      var item = el("li", { style: styles.item }, [
        meta,
        el("p", { style: styles.body, text: comment.content }),
      ]);
      if (comment.parentId) {
        item.style.marginLeft = "2rem";
      }
      list.appendChild(item);
    });
  }

  function renderForm() {
    formHost.textContent = "";
    if (!localStorage.getItem("token")) {
      formHost.appendChild(
        el("div", { style: styles.login }, [
          el("p", {
            style: "margin:0 0 .75rem;",
            text: t("loginPrompt"),
          }),
          el("a", {
            style: styles.button + "display:inline-block;text-decoration:none;",
            href: "/admin/login",
            text: t("loginBtn"),
          }),
        ]),
      );
      return;
    }

    var form = el("form", { style: styles.form });
    var textarea = el("textarea", {
      style: styles.textarea,
      placeholder: t("placeholder"),
      maxlength: "100",
    });
    var counter = el("span", { style: styles.counter, text: "0/100" });
    textarea.addEventListener("input", function () {
      counter.textContent = textarea.value.length + "/100";
    });
    var note = el("p", { style: styles.note });
    form.appendChild(textarea);
    form.appendChild(
      el("div", { style: styles.row }, [
        counter,
        el("button", {
          style: styles.button,
          type: "submit",
          text: t("submit"),
        }),
      ]),
    );
    form.appendChild(note);
    formHost.appendChild(form);

    form.addEventListener("submit", function (e) {
      e.preventDefault();
      var content = textarea.value.trim();
      if (!content) return;
      var btn = form.querySelector('button[type="submit"]');
      btn.disabled = true;
      note.textContent = "";
      fetch(API + "/comments", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer " + localStorage.getItem("token"),
        },
        body: JSON.stringify({ postId: Number(postId), content: content }),
      })
        .then(function (res) {
          return res.json().then(function (data) {
            return { ok: res.ok, data: data };
          });
        })
        .then(function (r) {
          if (!r.ok) {
            throw new Error(r.data.error || t("postError"));
          }
          textarea.value = "";
          counter.textContent = "0/100";
          if (r.data.requiresModeration) {
            note.textContent = t("moderationNote");
          }
          loadComments(renderList);
        })
        .catch(function (err) {
          note.style.color = "#b91c1c";
          note.textContent = err.message || t("postError");
        })
        .finally(function () {
          btn.disabled = false;
        });
    });
  }

  function deleteComment(comment) {
    if (!window.confirm(t("deleteConfirm"))) return;
    fetch(API + "/comments/" + comment.id, {
      method: "DELETE",
      headers: { Authorization: "Bearer " + localStorage.getItem("token") },
    })
      .then(function (res) {
        if (!res.ok) {
          return res.json().then(function (data) {
            throw new Error(data.error || t("deleteError"));
          });
        }
        return res.json();
      })
      .then(function () {
        loadComments(renderList);
      })
      .catch(function (err) {
        window.alert(err.message || t("deleteError"));
      });
  }

  function renderError() {
    list.textContent = "";
    list.appendChild(el("li", { style: styles.error, text: t("loadError") }));
  }

  // Wire the like button the theme places on the post page
  // (<button data-like-post-id="...">). Guest clicks send the user to the
  // login page; authenticated clicks toggle the like through the public API
  // and update the count and filled-heart state in place.
  function initLikeButton() {
    var btn = document.querySelector("[data-like-post-id]");
    if (!btn) return;
    var postId = btn.getAttribute("data-like-post-id");
    var countEl = btn.querySelector("[data-like-count]");
    // For signed-in readers, reflect the user's current like state on load.
    if (localStorage.getItem("token")) {
      fetch(API + "/likes/" + encodeURIComponent(postId))
        .then(function (res) {
          return res.ok ? res.json() : null;
        })
        .then(function (data) {
          if (!data) return;
          if (countEl) countEl.textContent = String(data.likesCount || 0);
          btn.setAttribute("aria-pressed", data.isLiked ? "true" : "false");
        })
        .catch(function () {});
    }
    btn.addEventListener("click", function () {
      if (!localStorage.getItem("token")) {
        window.location.href = "/admin/login";
        return;
      }
      fetch(API + "/likes/" + encodeURIComponent(postId), {
        method: "POST",
        headers: { Authorization: "Bearer " + localStorage.getItem("token") },
      })
        .then(function (res) {
          return res.json().then(function (data) {
            return { ok: res.ok, data: data };
          });
        })
        .then(function (r) {
          if (!r.ok) throw new Error(r.data.error || t("likeError"));
          if (countEl) countEl.textContent = String(r.data.likesCount || 0);
          btn.setAttribute("aria-pressed", r.data.isLiked ? "true" : "false");
        })
        .catch(function (err) {
          window.alert(err.message || t("likeError"));
        });
    });
  }

  // Wire the share button the theme places on the post page
  // (<button data-share-url="/post/...">): copying the absolute post URL to
  // the clipboard and briefly revealing the "Link copied" hint.
  function initShareButton() {
    var btn = document.querySelector("[data-share-url]");
    if (!btn) return;
    var path = btn.getAttribute("data-share-url") || "/";
    var hint = document.querySelector("[data-share-hint]");
    var showHint = function () {
      if (!hint) return;
      hint.hidden = false;
      setTimeout(function () {
        hint.hidden = true;
      }, 3000);
    };
    btn.addEventListener("click", function () {
      var url = window.location.origin + path;
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(url).then(showHint).catch(showHint);
      } else {
        showHint();
      }
    });
  }

  // Build the static section skeleton once; the list and form refresh in place.
  var section = el("section", { style: "margin-top:2rem;" });
  var title = el("h2", { style: styles.title, text: t("title") });
  var count = el("span", { style: styles.count, text: "" });
  title.appendChild(count);
  section.appendChild(title);
  var list = el("ul", { style: styles.list });
  section.appendChild(list);
  var formHost = el("div");
  section.appendChild(formHost);
  container.appendChild(section);

  loadComments(renderList);
  renderForm();
  initLikeButton();
  initShareButton();
})();
