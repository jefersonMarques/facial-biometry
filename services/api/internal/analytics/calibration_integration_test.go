package analytics

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "testing"
    "time"
)

func TestCalibrationTruthRequiresIndependentEvidence(t *testing.T) {
    for _, tc:=range []struct{label,source string; valid bool}{
        {"unknown","",true},
        {"genuine_confirmed","controlled_test",true},
        {"impostor_confirmed","external_verified",true},
        {"genuine_confirmed","",false},
        {"impostor_confirmed","",false},
        {"unknown","controlled_test",false},
        {"approved","controlled_test",false},
        {"impostor_confirmed","automatic_decision",false},
    } {
        err:=ValidateCalibrationTruth(tc.label,tc.source)
        if (err==nil)!=tc.valid {
            t.Fatalf("label %q, source %q: invalid result %v",tc.label,tc.source,err)
        }
    }
}

func TestCalibrationAggregatesRealPostgresEvents(t *testing.T) {
    url:=os.Getenv("FACEPROOF_ANALYTICS_TEST_DATABASE_URL")
    if url=="" {t.Skip("requires PostgreSQL test service")}
    ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second)
    defer cancel()
    r,err:=Open(ctx,url,make([]byte,32))
    if err!=nil {t.Fatal(err)}
    defer r.Close()
    now:=time.Now().UTC()
    ids:=[]string{}
    defer func(){
        for _,id:=range ids {
            _,_ = r.db.ExecContext(context.Background(),
                "DELETE FROM faceproof_checks WHERE check_id=$1",id)
        }
    }()
    for i:=0;i<3;i++ {
        id:=fmt.Sprintf("chk_cal_test_%d_%d",now.UnixNano(),i)
        ids=append(ids,id)
        decision:="approved"
        if i==1 {decision="rejected"}
        runtime:="r1"
        if i==2 {runtime="r2"}
        _,err=r.db.ExecContext(ctx,`INSERT INTO faceproof_checks (
           check_id,tenant_id,subject_hash,scenario,expected_decision,status,actual_decision,
           created_at,expires_at,completed_at,document,embedding_model,runtime
         ) VALUES($1,'integration','hash','unknown','',$2,$2,$3,$4,$3,'{}'::jsonb,'sface',$5::jsonb)`,
           id,decision,now.Add(-2*time.Minute),now.Add(time.Hour),
           `{"runtimeId":"`+runtime+`"}`)
        if err!=nil {t.Fatal(err)}
        if i==0 {
            if err=r.UpdateCalibrationTruth(ctx,id,"genuine_confirmed","controlled_test");err!=nil{t.Fatal(err)}
        } else if i==1 {
            if err=r.UpdateCalibrationTruth(ctx,id,"impostor_confirmed","controlled_test");err!=nil{t.Fatal(err)}
        }
        val:=[]float64{0.83,0.17,0.64}[i]
        vios:=[]float64{0.39,0.10,0.42}[i]
        payload:=func(pdfFace, vioFace float64) map[string]any {
            return map[string]any{
                "pdf_live":map[string]any{"status":"compared","robustCosine":pdfFace},
                "variants":[]any{
                    map[string]any{
                        "method":"production_cubic","status":"compared","durationMs":0,
                        "pdfVio":map[string]any{"status":"compared","robustCosine":0.42},
                        "vioLive":map[string]any{"status":"compared","robustCosine":vioFace},
                    },
                    map[string]any{
                        "method":"nearest","status":"analysis_failed","durationMs":24,
                        "errorCode":"reference_extraction_failed",
                    },
                },
            }
        }
        if i==0 {
            // Multiple telemetry events for one check must not double count.
            if err=r.RecordEvent(ctx,id,"cnh_low_resolution_shadow",payload(0.05,0.02));err!=nil{t.Fatal(err)}
        }
        if err=r.RecordEvent(ctx,id,"cnh_low_resolution_shadow",payload(val,vios));err!=nil{t.Fatal(err)}
    }

    report,err:=r.Calibration(ctx,now.Add(-time.Hour),now.Add(time.Hour),"")
    if err!=nil {t.Fatal(err)}
    if report.CompletedCNH!=3 || report.WithBenchmark!=3 ||
       report.ConfirmedGenuine!=1 || report.ConfirmedImpostor!=1 ||
       report.UnknownTruth!=1 || report.Approved!=2 || report.Rejected!=1 ||
       report.Calibrated {
        t.Fatalf("unexpected report counts: %+v",report)
    }
    for _,tc:=range []struct{truth string; expected float64}{
        {"genuine_confirmed",0.39},
        {"impostor_confirmed",0.10},
        {"unknown",0.42},
    } {
        found:=false
        for _,item:=range report.Series {
            if item.Method=="production_cubic" && item.Pair=="vio_live" &&
               item.TruthLabel==tc.truth {
                found=true
                if item.Attempts!=1 || item.Compared!=1 || item.MeanCosine==nil ||
                   *item.MeanCosine < tc.expected-0.00001 || *item.MeanCosine > tc.expected+0.00001 {
                    t.Fatalf("wrong cohort %+v",item)
                }
            }
        }
        if !found {t.Fatalf("missing cohort %q",tc.truth)}
    }
    foundNearest:=false
    for _,method:=range report.MethodHealth {
        if method.Method=="nearest" {
            foundNearest=true
            if method.Attempts!=3 || method.Failed!=3 || method.Compared!=0 {
                t.Fatalf("nearest extraction failures not counted: %+v",method)
            }
        }
    }
    if len(report.MethodHealth)!=2 || !foundNearest {
        t.Fatalf("missing interpolation diagnostics: %+v",report.MethodHealth)
    }
    filtered,err:=r.Calibration(ctx,now.Add(-time.Hour),now.Add(time.Hour),"r1")
    if err!=nil {t.Fatal(err)}
    if filtered.CompletedCNH!=2 || filtered.WithBenchmark!=2 {
        t.Fatalf("runtime filter mixed model samples: %+v",filtered)
    }
    if err=r.UpdateCalibrationTruth(ctx,ids[0],"unknown","");err!=nil{t.Fatal(err)}
    label,source,err:=r.CalibrationTruth(ctx,ids[0])
    if err!=nil || label!="unknown" || source!="" {t.Fatalf("truth reset failed: %s %s %v",label,source,err)}
    var audits int
    if err=r.db.QueryRowContext(ctx,`SELECT COUNT(*) FROM faceproof_events
       WHERE check_id=$1 AND event_type='admin_ground_truth_updated'`,ids[0]).Scan(&audits);err!=nil{t.Fatal(err)}
    if audits!=2 {t.Fatalf("expected immutable audit of both changes, got %d",audits)}
    raw,err:=json.Marshal(report)
    if err!=nil {t.Fatal(err)}
    for _,needle:=range []string{ids[0],"subject_hash","payload","embedding"} {
        // "embeddingModel" in runtime aggregate is permitted; never leak an embedding vector.
        if needle=="embedding" {continue}
        if string(raw)!="" && containsBytes(raw,[]byte(needle)) {
            t.Fatalf("aggregate leaked restricted value %q",needle)
        }
    }
}

func containsBytes(haystack,needle []byte)bool {
    for offset:=0;offset+len(needle)<=len(haystack);offset++ {
        equal:=true
        for i:=range needle {if haystack[offset+i]!=needle[i]{equal=false;break}}
        if equal{return true}
    }
    return false
}
