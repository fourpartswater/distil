import Vue from 'vue';
import VueRouter from 'vue-router';
import VueRouterSync from 'vuex-router-sync';
import Home from './views/Home.vue';
import Search from './views/Search.vue';
import Select from './views/Select.vue';
import Results from './views/Results.vue';
import Navigation from './views/Navigation.vue';
import { getters as routeGetters } from './store/route/module';
import { mutations as viewMutations } from './store/view/module';
import store from './store/store';
import BootstrapVue from 'bootstrap-vue';

import './assets/favicons/apple-touch-icon.png';
import './assets/favicons/favicon-32x32.png';
import './assets/favicons/favicon-16x16.png';
import './assets/favicons/manifest.json';
import './assets/favicons/safari-pinned-tab.svg';

import 'bootstrap-vue/dist/bootstrap-vue.css';

import './styles/bootstrap-v4beta2-custom.css';
import './styles/main.css';

Vue.use(VueRouter);
Vue.use(BootstrapVue);

const router = new VueRouter({
	routes: [
		{ path: '/', redirect: '/home' },
		{ path: '/home', component: Home },
		{ path: '/search', component: Search },
		{ path: '/select', component: Select },
		{ path: '/results', component: Results }
	]
});

router.beforeEach((route, _, next) => {
	const dataset = route.query ? route.query.dataset : routeGetters.getRouteDataset(store);
	viewMutations.saveView(store, {
		view: route.path,
		dataset: dataset,
		route: route
	});
	next();
});

// sync store and router
VueRouterSync.sync(store, router, { moduleName: 'routeModule' });

// init app
new Vue({
	store,
	router,
	components: {
		Navigation
	},
	template: `
		<div id="distil-app">
			<navigation/>
			<router-view class="view"></router-view>
		</div>`
}).$mount('#app');
