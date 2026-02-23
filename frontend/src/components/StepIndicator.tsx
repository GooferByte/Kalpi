'use client'

const STEPS = [
  { n: 1, label: 'Connect Broker' },
  { n: 2, label: 'Build Portfolio' },
  { n: 3, label: 'Execute' },
  { n: 4, label: 'Results' },
]

export default function StepIndicator({ currentStep }: { currentStep: number }) {
  return (
    <div className="flex items-center justify-center mb-10">
      {STEPS.map((step, i) => (
        <div key={step.n} className="flex items-center">
          {/* Circle */}
          <div className="flex flex-col items-center gap-1.5">
            <div
              className={`w-9 h-9 rounded-full flex items-center justify-center text-sm font-bold transition-all ${
                currentStep > step.n
                  ? 'bg-emerald-600 text-white'
                  : currentStep === step.n
                  ? 'bg-emerald-600 text-white ring-4 ring-emerald-100'
                  : 'bg-slate-100 text-slate-400'
              }`}
            >
              {currentStep > step.n ? (
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                </svg>
              ) : (
                step.n
              )}
            </div>
            <span
              className={`text-xs font-medium whitespace-nowrap ${
                currentStep >= step.n ? 'text-emerald-700' : 'text-slate-400'
              }`}
            >
              {step.label}
            </span>
          </div>
          {/* Connector line */}
          {i < STEPS.length - 1 && (
            <div
              className={`h-0.5 w-16 mx-2 mb-5 transition-all ${
                currentStep > step.n ? 'bg-emerald-500' : 'bg-slate-200'
              }`}
            />
          )}
        </div>
      ))}
    </div>
  )
}
