//go:build kyse

package matrix

import (
	"github.com/arandu-io/kyse/components"

	permission "github.com/hyz-is/arandu-permission"
)

@go
// SummaryData is what the handler hands this fragment.
type SummaryData = permission.SummaryPageData
@endgo

{{-- This extends no layout. It is swapped into a page that already has one,
     and a fragment that extended one would put a second copy of the header and
     the navigation inside the element it was swapped into. --}}

<div class="rounded-md border p-4">
	<h3 class="text-sm font-semibold tracking-tight">Before this is written</h3>

	@if(.Change.Empty())
		<p class="text-muted-foreground mt-2 text-sm">Nothing would change.</p>
	@else
		<dl class="mt-3 grid gap-3 text-sm">
			@if(len(.Change.Added) > 0)
				<div>
					<dt class="font-medium">Added</dt>
					<dd class="mt-1 flex flex-wrap gap-1">
						@foreach(.Change.Added as value)
							{!! components.Badge(components.BadgeProps{Label: value}) !!}
						@endforeach
					</dd>
				</div>
			@endif
			@if(len(.Change.Removed) > 0)
				<div>
					<dt class="font-medium">Removed</dt>
					<dd class="mt-1 flex flex-wrap gap-1">
						@foreach(.Change.Removed as value)
							{!! components.Badge(components.BadgeProps{Label: value, Variant: "destructive"}) !!}
						@endforeach
					</dd>
				</div>
			@endif
			@if(len(.Change.Unchanged) > 0)
				<div>
					<dt class="text-muted-foreground font-medium">Unchanged</dt>
					<dd class="text-muted-foreground mt-1 text-xs">{{ len(.Change.Unchanged) }} kept as they are</dd>
				</div>
			@endif
		</dl>

		{{-- The confirmation carries the same values that produced the summary,
		     so approving it sends the request that was described rather than
		     whatever the form holds by then. --}}
		<form class="mt-4" hx-put="{{ .Target }}">
			@csrf
			<input type="hidden" name="kind" value="{{ .Kind }}">
			@foreach(.Fields as field)
				<input type="hidden" name="value" value="{{ field }}">
			@endforeach
			{!! components.Button(components.ButtonProps{Label: "Apply this change", Type: "submit"}) !!}
		</form>
	@endif
</div>
