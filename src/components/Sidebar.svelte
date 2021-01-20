<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { fire, post } from "../misc";
  import {
    category,
    component,
    showSidebar,
    loading,
    categories,
  } from "../stores";
  import type { Category } from "../stores";

  const dispatch = createEventDispatcher();
  const isSmall = 900;

  let hover = false;
  let smallSize = window.innerWidth <= isSmall;

  const toggle = () => {
    $showSidebar = !$showSidebar;
  };

  const goto = (c: Category) => {
    if (window.innerWidth <= isSmall) $showSidebar = false;
    $category = c;
    $component = "show";
  };

  const add = async (category: string) => {
    category = category.trim();
    $loading++;
    const resp = await post("/category/add", { category });
    $loading--;
    let json: any = {};
    if (resp.ok) {
      json = await resp.json();
      if (json.id && json.status) {
        (document.querySelector(".new") as Element).remove();
        const newList: Category = {
          id: json.id,
          category,
          count: 0,
        };
        $categories = [...$categories, newList];
        goto(newList);
        return true;
      }
    }
    await fire("Error", json.message ? json.message : "Error", "error");
    dispatch("reload");
    return false;
  };

  const addCategory = async () => {
    if (window.innerWidth <= isSmall) $showSidebar = false;
    const newList = document.querySelector(".new");
    let ok = true;
    if (newList) ok = await add((newList as HTMLElement).innerText);
    if (ok) {
      const ul = document.querySelector("ul.navbar-nav") as Element;
      const li = document.createElement("li");
      li.classList.add("nav-link", "new");
      ul.appendChild(li);
      li.addEventListener("keydown", async (event) => {
        const target = event.target as Element;
        const list = (target.textContent as string).trim();
        if (event.key == "Enter") {
          event.preventDefault();
          if (list) await add(list);
          else target.remove();
        } else if (event.key == "Escape") {
          if (list) target.textContent = "";
          else target.remove();
        }
      });
      li.setAttribute("contenteditable", "true");
      li.focus();
      const range = document.createRange();
      range.selectNodeContents(li);
      range.collapse(false);
      const sel = window.getSelection() as Selection;
      sel.removeAllRanges();
      sel.addRange(range);
    }
  };

  const checkSize = () => {
    if (smallSize != window.innerWidth <= isSmall)
      smallSize = window.innerWidth <= isSmall;
  };
  const handleKeydown = async (event: KeyboardEvent) => {
    if (event.key == "ArrowUp" || event.key == "ArrowDown") {
      const newCategory = document.querySelector(".new");
      if (newCategory) newCategory.remove();
      const len = $categories.length;
      const index = $categories.findIndex((c) => c.id === $category.id);
      if ($category.id && $component === "show")
        if (event.key == "ArrowUp") {
          if (index > 0) goto($categories[index - 1]);
        } else if (event.key == "ArrowDown")
          if (index < len - 1) goto($categories[index + 1]);
    }
  };
  const handleClick = async (event: MouseEvent) => {
    const target = event.target as Element;
    if (
      !target.classList.contains("new") &&
      !target.classList.contains("swal2-confirm") &&
      target.textContent !== "Add Category"
    ) {
      const newCategory = document.querySelector(".new");
      if (newCategory) {
        const list = (newCategory.textContent as string).trim();
        if (list) await add(list);
        else newCategory.remove();
      }
    }
  };
</script>

<svelte:window
  on:keydown={handleKeydown}
  on:resize={checkSize}
  on:click={handleClick}
/>

{#if smallSize}
  <span
    class="toggle"
    on:click={toggle}
    on:mouseenter={() => (hover = true)}
    on:mouseleave={() => (hover = false)}>
    <svg viewBox="0 0 70 70" width="40" height="30">
      {#each [10, 30, 50] as y}
        <rect {y} width="100%" height="10" fill={hover ? "#1a73e8" : "white"} />
      {/each}
    </svg>
  </span>
{/if}
<nav
  class="nav flex-column navbar-light sidebar"
  hidden={!$showSidebar && smallSize}
>
  <div class="category-menu">
    <button class="btn btn-primary btn-sm" on:click={addCategory}>
      Add Category
    </button>
    <ul class="navbar-nav" id="categories">
      <li
        class="navbar-brand category"
        class:active={$category.id === -1 && $component === "show"}
        on:click={() => goto({ id: -1, category: "All Bookmarks", count: 0 })}
      >All Bookmarks ()</li>
      {#each $categories as c (c.id)}
        <li
          class="nav-link category"
          class:active={$category.id === c.id && $component === "show"}
          on:click={() => goto(c)}
        >
          {c.category} ({c.count})
        </li>
      {/each}
    </ul>
  </div>
</nav>

<style>
  .toggle {
    position: fixed;
    z-index: 100;
    top: 0;
    padding: 20px;
    color: white !important;
  }

  .toggle:hover {
    background-color: rgb(232, 232, 232);
  }

  .sidebar {
    position: fixed;
    top: 0;
    z-index: 1;
    height: 100%;
    width: 250px;
    padding-top: 70px;
    user-select: none;
  }

  .category-menu {
    height: 100%;
    width: 100%;
    padding-top: 10px;
    overflow-x: hidden;
    border-right: 1px solid #e9ecef;
    background-color: white;
  }

  .category-menu .btn {
    margin-left: 20px;
    margin-bottom: 5px;
  }

  .category-menu .navbar-brand {
    text-indent: 10px;
  }

  .category-menu .navbar-nav {
    text-indent: 20px;
  }

  .category-menu .nav-link:hover {
    background-color: rgb(232, 232, 232);
  }

  #categories {
    height: calc(100% - 76px);
    overflow-y: auto;
  }

  .category {
    display: block;
    cursor: pointer;
    margin: 0;
    border-left: 5px solid transparent;
    color: rgba(0, 0, 0, 0.7) !important;
  }

  .active {
    border-left: 5px solid #1a73e8;
    color: #1a73e8 !important;
  }

  .nav-link.active {
    background-color: #eaf5fd;
  }

  @media (min-width: 901px) {
    .sidebar {
      display: block !important;
    }
  }

  @media (max-width: 900px) {
    .sidebar {
      box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
    }
  }
</style>