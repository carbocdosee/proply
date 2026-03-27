<script lang="ts">
	// Auth layout: centered card, no sidebar
</script>

<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
	<div class="w-full max-w-md">
		<div class="text-center mb-8">
			<a href="/" class="text-2xl font-bold text-indigo-600">Proply</a>
		</div>
		<div class="bg-white rounded-2xl shadow-sm border border-gray-100 p-8">
			<slot />
		</div>
	</div>
</div>
