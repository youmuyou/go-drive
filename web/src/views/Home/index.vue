<template>
  <div class="home">
    <!-- file list main area -->
    <div class="files-list">
      <entry-list-view
        ref="entryList"
        :path="path"
        @entries-load="entriesLoaded"
        @entry-click="entryClicked"
        @entry-menu="showEntryMenu"
        :selection.sync="selectedEntries"
        @loading="progressBar($event)"
        :view-mode="viewMode"
      />
    </div>
    <!-- file list main area -->

    <!-- README -->
    <div class="page-footer" v-if="readmeContent">
      <div class="markdown-body" v-markdown="readmeContent"></div>
    </div>
    <!-- README -->

    <!-- entry handler view dialog -->
    <dialog-view class="entry-handler-dialog" :show="entryHandlerViewShowing">
      <component
        v-if="entryHandlerViewShowing"
        :is="entryHandlerView.component"
        :entry="entryHandlerView.entry"
        :entries="entries"
        @update="reloadEntryList"
        @close="closeEntryHandlerView"
        @entry-change="entryHandlerViewChange"
        @save-state="entryHandlerViewSaveStateChange"
      />
    </dialog-view>
    <!-- entry handler view dialog -->

    <!-- entry menu -->
    <dialog-view
      v-model="entryMenuShowing"
      overlay-close
      esc-close
      transition="flip-fade"
    >
      <entry-menu
        v-if="entryMenu"
        :menus="entryMenu.menus"
        :entry="entryMenu.entry"
        @click="menuClicked"
      />
    </dialog-view>
    <!-- entry menu -->

    <!-- new entry menu -->
    <new-entry-area
      ref="newEntryArea"
      :path="path"
      :entries="entries"
      @update="reloadEntryList"
      :readonly="isCurrentDirReadonly"
    />
    <!-- new entry menu -->
  </div>
</template>
<script>
import EntryListView from '@/views/EntryListView'
import EntryMenu from './EntryMenu'
import NewEntryArea from './NewEntryArea'

import { getEntry, getContent } from '@/api'
import { filename, dir, debounce, supportThumbnail, setTitle } from '@/utils'

import { resolveEntryHandler, HANDLER_COMPONENTS, getHandler } from '@/utils/handlers'
import { makeEntryHandlerLink, getBaseLink } from '@/utils/routes'
import { mapMutations, mapState } from 'vuex'

const README_FILENAME = 'readme.md'

const HISTORY_FLAG = '_h'
const setHistoryFlag = () => {
  sessionStorage.setItem(HISTORY_FLAG, '1')
}
const getHistoryFlag = () => {
  const val = sessionStorage.getItem(HISTORY_FLAG)
  sessionStorage.removeItem(HISTORY_FLAG)
  return !!val
}

export default {
  name: 'Home',
  components: { EntryListView, EntryMenu, NewEntryArea, ...HANDLER_COMPONENTS },
  props: {
    path: {
      type: String,
      required: true
    }
  },
  data () {
    return {
      readmeContent: '',

      entryHandlerView: null,

      entryMenu: null,
      entryMenuShowing: false,

      entries: null,
      selectedEntries: [],

      currentDirEntry: null,

      viewMode: 'line'
    }
  },
  computed: {
    entryHandlerViewShowing () {
      return !!(this.entryHandlerView && this.entries && this.entryHandlerView.entry)
    },
    isCurrentDirReadonly () {
      return !this.currentDirEntry || !this.currentDirEntry.meta.can_write
    },
    ...mapState(['user'])
  },
  beforeRouteUpdate (to, from, next) {
    if (!this.resolveHandlerByRoute(from) && this.resolveHandlerByRoute(to)) {
      setHistoryFlag()
    }
    this.confirmUnsavedState().then(() => {
      next()
      this.resolveRouteAndHandleEntry()
    }, () => {
      next(false)
    })
  },
  beforeRouteLeave (to, from, next) {
    this.progressBar(false)
    next()
  },
  created () {
    this.reloadEntryList = debounce(this.reloadEntryList, 500)
    window.addEventListener('beforeunload', this.onWindowUnload)
    window.addEventListener('keydown', this.onKeyDown)
    this.resolveRouteAndHandleEntry()
  },
  beforeDestroy () {
    window.removeEventListener('keydown', this.onKeyDown)
    window.removeEventListener('beforeunload', this.onWindowUnload)
  },
  methods: {
    entryClicked ({ entry, event }) {
      if (this.selectedEntries.length > 0) {
        this.selectedEntries.splice(0)
        event.preventDefault()
        return
      }
      if (entry.type === 'dir') {
        // path changed
        this.entries = null
        this.currentDirEntry = null
      }
    },
    menuClicked ({ entry, menu }) {
      this.entryMenuShowing = false
      const handler = getHandler(menu.name)
      if (!handler) return

      if (handler.view) {
        if (Array.isArray(entry)) { // selected entries
          this.entryHandlerView = {
            handler: handler.name,
            component: handler.view && handler.view.name, entry,
            savedState: true
          }
        } else {
          this.$router.push(makeEntryHandlerLink(handler.name, entry.name, this.path))
        }
        return
      }
      // execute handler
      if (typeof (handler.handler) === 'function') {
        handler.handler(entry, this.$uiUtils).then(r => {
          if (r && r.update) this.reloadEntryList()
        }, () => { })
      }
    },
    showEntryMenu ({ entry, event }) {
      if (this.selectedEntries.length > 0) {
        entry = [...this.selectedEntries] // selected entries
      }
      const handlers = resolveEntryHandler(entry, this.currentDirEntry, this.user)
      if (handlers.length === 0) return

      event && event.preventDefault()

      this.entryMenu = {
        entry,
        menus: handlers
          .filter(h => h.display)
          .map(h => ({
            name: h.name,
            display: typeof (h.display) === 'function' ? h.display(entry) : h.display
          }))
      }
      this.entryMenuShowing = true
    },
    entriesLoaded ({ entries, path }) {
      this.viewMode = entries.reduce((n, e) =>
        n + +supportThumbnail(e), 0) / entries.length > 0.5
        ? 'block' : 'line'

      setTitle(path)

      if (path !== this.path) {
        this.$router.push(getBaseLink(path))
      }
      this.tryLoadReadme(entries)
      this.entries = entries
      this.selectedEntries.splice(0)

      if (this.entryHandlerView && this.entryHandlerView.entryName) {
        this.entryHandlerView.entry =
          entries.find(e => e.name === this.entryHandlerView.entryName)

        setTitle(`${this.entryHandlerView.entryName}`)
      }

      // load current path
      if (this._getEntryTask) this._getEntryTask.cancel()
      this._getEntryTask = getEntry(path).then(entry => { this.currentDirEntry = entry }, () => { })
    },
    resolveRouteAndHandleEntry () {
      const matched = this.resolveHandlerByRoute(this.$route)
      if (!matched) {
        this.entryHandlerView = null
        return false
      }
      const { handler, entryName } = matched
      const entry = this.entries && this.entries.find(e => e.name === entryName)

      if (handler.view) {
        // handler view dialog
        this.entryHandlerView = {
          handler: handler.name,
          component: handler.view && handler.view.name,
          entryName, entry,
          savedState: true
        }

        setTitle(`${entryName}`)
      }
    },
    async closeEntryHandlerView () {
      setTitle(this.path)

      if (!this.entryHandlerView) return
      if (this.entryHandlerView.entryName) {
        this.focusOnEntry(this.entryHandlerView.entryName)
      }
      if (!this.replaceHandlerRoute()) {
        this.entryHandlerView = null
      }
    },
    async entryHandlerViewChange (path) {
      try { await this.confirmUnsavedState() } catch { return }
      const dirPath = dir(path)
      const name = filename(path)
      this.focusOnEntry(name)
      const newPath = makeEntryHandlerLink(this.entryHandlerView.handler, name, dirPath)
      if (decodeURIComponent(this.$route.fullPath) !== decodeURIComponent(newPath)) {
        this.$router.replace(newPath)
      }
    },
    entryHandlerViewSaveStateChange (saved) {
      this.entryHandlerView.savedState = saved
    },
    confirmUnsavedState () {
      if (!this.entryHandlerView || this.entryHandlerView.savedState) return Promise.resolve()
      return this.$confirm(this.$t('p.home.unsaved_confirm'))
    },
    onWindowUnload (e) {
      if (!this.entryHandlerView || this.entryHandlerView.savedState) return
      e.preventDefault()
      e.returnValue = ''
    },
    resolveHandlerByRoute (route) {
      const handler = getHandler(route.query.handler)
      const entryName = route.query.entry
      if (!handler || !entryName) {
        return null
      }
      return { handler, entryName }
    },
    replaceHandlerRoute () {
      if (getHistoryFlag()) {
        this.$router.go(-1)
        return true
      } else {
        if (this.$route.fullPath !== this.$route.path) {
          this.$router.replace(this.$route.path)
          return true
        }
      }
    },
    focusOnEntry (name) {
      this.$refs.entryList.focusOnEntry(name)
    },
    getHandlerViewEntry (name) {
      return this.entries.find(e => e.name === name)
    },
    async tryLoadReadme (entries) {
      let readmeFound
      for (const e of entries) {
        if (e.type !== 'file') continue
        if (README_FILENAME.toLowerCase() === e.name.toLowerCase()) {
          readmeFound = e
          break
        }
      }
      if (readmeFound) {
        await this.loadReadme(readmeFound)
      } else {
        this.readmeContent = ''
      }
    },
    async loadReadme (entry) {
      if (this._readmeTask) this._readmeTask.cancel()
      let content
      this.readmeContent = `<p style="text-align: center">${this.$t('p.home.readme_loading')}</p>`
      this._readmeTask = getContent(entry.path, entry.meta.access_key)
      try {
        content = await this._readmeTask
      } catch (e) {
        if (e.isCancel) return
        content = `<p style="text-align: center;">${this.$t('p.home.readme_failed')}</p>`
      }
      if (this.path === dir(entry.path)) {
        this.readmeContent = content
      }
    },
    reloadEntryList () {
      this.selectedEntries.splice(0)
      this.$refs.entryList.reload()
    },
    onKeyDown (e) {
      if (e.key === 'Escape') {
        this.closeEntryHandlerView()
        e.stopPropagation()
        e.preventDefault()
      }
    },
    toggleViewMode () {
      if (this.viewMode === 'line') this.viewMode = 'block'
      else this.viewMode = 'line'
    },
    ...mapMutations(['progressBar'])
  }
}
</script>
<style lang="scss">
.home {
  margin-bottom: 40px;
}

.files-list {
  max-width: 880px;
  margin: 16px auto 0;
  @include var(background-color, primary-bg-color);
  padding: 16px 0;
  border-radius: 16px;
}

.page-footer {
  box-sizing: border-box;
  max-width: 880px;
  margin: 42px auto;
  @include var(background-color, primary-bg-color);
  padding: 16px;
  border-radius: 16px;
}

.entry-handler-dialog {
  .dialog-view__content {
    background-color: transparent;
  }
}

@media screen and (max-width: 900px) {
  .home {
    margin: 16px;
  }
  .entry-item__modified-time {
    display: none;
  }
}
</style>
