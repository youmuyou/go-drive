import Vue from 'vue'
import VueRouter from 'vue-router'

import Home from '@/views/Home'

Vue.use(VueRouter)

const routes = [
  {
    name: 'Index',
    path: '/',
    redirect: '/files/'
  },
  {
    name: 'Home',
    path: '/files/:path(.*)',
    component: Home,
    props: true
  },
  {
    name: 'TextEditor',
    path: '/editor/:path(.*)',
    component: () => import('@/views/TextEditPage')
  }
]

const router = new VueRouter({
  routes
})

export default router
