package org.example;

//TIP 要<b>运行</b>代码，请按 <shortcut actionId="Run"/> 或
// 点击装订区域中的 <icon src="AllIcons.Actions.Execute"/> 图标。


import org.mybatis.spring.annotation.MapperScan;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@MapperScan("org.example.demo.mapper")
@SpringBootApplication
public class MpCodegenDemoApplication {
    public static void main(String[] args){

            SpringApplication.run(MpCodegenDemoApplication.class, args);

    }

}